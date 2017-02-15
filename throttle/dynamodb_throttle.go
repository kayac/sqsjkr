package throttle

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

// DynamodbThrottle struct
type DynamodbThrottle struct {
	TableName       string
	Dynamodb        *dynamodb.DynamoDB
	RetentionPeriod time.Duration
}

// Set check double message
func (dt DynamodbThrottle) Set(jobid string) error {
	// DynamoDB's expression attribute and placeholders name or values:
	// http://docs.aws.amazon.com/amazondynamodb/latest/developerguide/ExpressionPlaceholders.html
	//
	//   Use the '#' character to dereference an attribute name.
	//   Tokens that begin with the ':' character are expression attribute values.
	//
	// ConditionExpression:
	// http://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Expressions.SpecifyingConditions.html
	expiredTime := strconv.FormatInt((time.Now().Add(dt.RetentionPeriod)).Unix(), 10)
	params := &dynamodb.UpdateItemInput{
		TableName: aws.String(dt.TableName),

		Key: map[string]*dynamodb.AttributeValue{
			"Id": {
				S: aws.String(jobid),
			},
			"Type": {
				S: aws.String("throttle"),
			},
		},

		ExpressionAttributeNames: map[string]*string{
			"#expired": aws.String("Expired"),
		},

		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":Expired": {
				N: aws.String(expiredTime),
			},
		},
		ConditionExpression: aws.String("attribute_not_exists(Expired)"),
		UpdateExpression:    aws.String("set #expired = :Expired"),

		ReturnConsumedCapacity:      aws.String("NONE"),
		ReturnItemCollectionMetrics: aws.String("NONE"),
		ReturnValues:                aws.String("NONE"),
	}

	_, err := dt.Dynamodb.UpdateItem(params)

	if err == nil {
		return nil
	}

	if awsErr, ok := err.(awserr.Error); ok {
		if awsErr.Code() == "ConditionalCheckFailedException" {
			return ErrDuplicatedMessage
		}
	}

	return err
}

// Unset delete record from dynamodb
func (dt DynamodbThrottle) Unset(jobid string) error {
	params := &dynamodb.DeleteItemInput{
		TableName: aws.String(dt.TableName),

		Key: map[string]*dynamodb.AttributeValue{
			"Id": {
				S: aws.String(jobid),
			},
			"Type": {
				S: aws.String("throttle"),
			},
		},

		ReturnConsumedCapacity:      aws.String("NONE"),
		ReturnItemCollectionMetrics: aws.String("NONE"),
		ReturnValues:                aws.String("NONE"),
	}

	_, err := dt.Dynamodb.DeleteItem(params)

	return err
}

// GetExpiredItems get items expired already.
func (dt DynamodbThrottle) GetExpiredItems() (*dynamodb.QueryOutput, error) {
	now := strconv.FormatInt(time.Now().Unix(), 10)

	params := &dynamodb.QueryInput{
		TableName: aws.String(dt.TableName),
		IndexName: aws.String(GlobalSecondaryIndex),

		// defines #expired as the Expired attribute of 'dt.TableName'.
		ExpressionAttributeNames: map[string]*string{
			"#type":    aws.String("Type"),
			"#expired": aws.String("Expired"),
		},

		// :Expired is the current unix timestamp 'now'.
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":Type": {
				S: aws.String("throttle"),
			},
			":Expired": {
				N: aws.String(now),
			},
		},

		// Filters items by `Expired <= now` from throttle items.
		// `Expired` values was defined at `Set(jobid string)`, which has `ThrottleExpired`
		// period from inserted unix time.
		KeyConditionExpression: aws.String("#type = :Type and #expired <= :Expired"),

		ReturnConsumedCapacity: aws.String("NONE"),
	}

	return dt.Dynamodb.Query(params)
}

// GetWriteCapacity get the table of write capacity unit.
func (dt *DynamodbThrottle) GetWriteCapacity() (int64, error) {
	params := &dynamodb.DescribeTableInput{
		TableName: aws.String(dt.TableName),
	}

	out, err := dt.Dynamodb.DescribeTable(params)
	if err != nil {
		return 0, err
	}

	return *out.Table.ProvisionedThroughput.WriteCapacityUnits, nil
}

// NewDynamodbThrottle build DynamodbThrottle
func NewDynamodbThrottle(ctx context.Context, profile, region, table string, retention time.Duration) Throttler {
	cred := credentials.NewSharedCredentials("", profile)

	// configure Dynamodb
	ddb := dynamodb.New(session.New(), &aws.Config{
		Region:      aws.String(region),
		Credentials: cred,
	})

	dt := &DynamodbThrottle{
		TableName:       table,
		Dynamodb:        ddb,
		RetentionPeriod: retention,
	}

	go DeleteTicker(ctx, dt)

	return dt
}

// DeleteTicker delete items of the previous day every hour
func DeleteTicker(ctx context.Context, dt *DynamodbThrottle) {
	ticker := time.NewTicker(DeleteTickerPeriod)

	// use one fifth of write capacity when to delete
	wcu, err := dt.GetWriteCapacity()
	if wcu < DeleteCapacityRate {
		wcu = 1
	} else {
		wcu = wcu / DeleteCapacityRate
	}

	if err != nil {
		panic(err)
	}

	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			out, err := dt.GetExpiredItems()
			if err != nil {
				// TODO: use logger
				fmt.Println(err.Error())
				continue
			}

			// Unset all expired item
			for {
				// Number of unsetting items at one time should be
				// defiend based on write unit capacities for
				// the DynamoDB table.
				var dels []map[string]*dynamodb.AttributeValue
				if int64(len(out.Items)) < wcu {
					dels = out.Items
					out.Items = out.Items[len(out.Items):]
				} else {
					dels = out.Items[:wcu]
					out.Items = out.Items[wcu:]
				}

				// Unset items following with write unit capacity.
				for _, item := range dels {
					jobid := *item["Id"].S
					if err := dt.Unset(jobid); err != nil {
						// TODO: use logger
						fmt.Println(err)
						out.Items = append(out.Items, item)
					}
				}

				if len(out.Items) == 0 {
					break
				}

				time.Sleep(time.Second * 1)
			}
		}
	}
}
