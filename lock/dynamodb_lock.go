package lock

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

// DynamodbLock is to lock job and check duplicated message
type DynamodbLock struct {
	TableName string
	dynamodb  *dynamodb.DynamoDB
}

// Lock put a record into dynamodb with lock_id
func (dl DynamodbLock) Lock(lockID, eventID string) error {
	// DynamoDB's expression attribute and placeholders name or values:
	// http://docs.aws.amazon.com/amazondynamodb/latest/developerguide/ExpressionPlaceholders.html
	//
	//   Use the '#' character to dereference an attribute name.
	//   Tokens that begin with the ':' character are expression attribute values.
	//
	// ConditionExpression:
	// http://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Expressions.SpecifyingConditions.html
	param := &dynamodb.UpdateItemInput{
		TableName: aws.String(dl.TableName),

		Key: map[string]*dynamodb.AttributeValue{
			"Id": {
				S: aws.String(lockID),
			},
			"Type": {
				S: aws.String("lock"),
			},
		},

		ExpressionAttributeNames: map[string]*string{
			"#eventid": aws.String("EventId"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":EventId": {
				S: aws.String(eventID),
			},
		},
		ConditionExpression: aws.String("attribute_not_exists(EventId)"),
		UpdateExpression:    aws.String("set #eventid = :EventId"),

		ReturnConsumedCapacity:      aws.String("NONE"),
		ReturnItemCollectionMetrics: aws.String("NONE"),
		ReturnValues:                aws.String("NONE"),
	}

	_, err := dl.dynamodb.UpdateItem(param)

	if err == nil {
		return nil
	}

	if awsErr, ok := err.(awserr.Error); ok {
		if awsErr.Code() == "ConditionalCheckFailedException" {
			return fmt.Errorf("Already '%s' is locked, error_code:%s, reason:%s",
				lockID,
				awsErr.Code(),
				awsErr.Message(),
			)
		}
	}

	return err
}

// Unlock delete record from dynamodb with lock_id
func (dl DynamodbLock) Unlock(lockID string) error {
	params := &dynamodb.DeleteItemInput{
		TableName: aws.String(dl.TableName),

		Key: map[string]*dynamodb.AttributeValue{
			"Id": {
				S: aws.String(lockID),
			},
			"Type": {
				S: aws.String("lock"),
			},
		},

		ReturnConsumedCapacity:      aws.String("NONE"),
		ReturnItemCollectionMetrics: aws.String("NONE"),
		ReturnValues:                aws.String("NONE"),
	}

	_, err := dl.dynamodb.DeleteItem(params)

	return err
}

// NewDynamodbLock build DynamodbLock
func NewDynamodbLock(profile, region, table string) Locker {
	cred := credentials.NewSharedCredentials("", profile)
	// configure Dynamodb
	ddb := dynamodb.New(session.New(), &aws.Config{
		Region:      &region,
		Credentials: cred,
	})
	return DynamodbLock{
		TableName: table,
		dynamodb:  ddb,
	}
}
