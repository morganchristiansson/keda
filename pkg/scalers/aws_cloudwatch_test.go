package scalers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	testAWSCloudwatchRoleArn         = "none"
	testAWSCloudwatchAccessKeyID     = "none"
	testAWSCloudwatchSecretAccessKey = "none"
	testAWSCloudwatchErrorMetric     = "Error"
	testAWSCloudwatchNoValueMetric   = "NoValue"
)

var testAWSCloudwatchResolvedEnv = map[string]string{
	"AWS_ACCESS_KEY":        "none",
	"AWS_SECRET_ACCESS_KEY": "none",
}

var testAWSAuthentication = map[string]string{
	"awsAccessKeyId":     testAWSCloudwatchAccessKeyID,
	"awsSecretAccessKey": testAWSCloudwatchSecretAccessKey,
}

type parseAWSCloudwatchMetadataTestData struct {
	metadata   map[string]string
	authParams map[string]string
	isError    bool
	comment    string
}

type awsCloudwatchMetricIdentifier struct {
	metadataTestData *parseAWSCloudwatchMetadataTestData
	scalerIndex      int
	name             string
}

var testAWSCloudwatchMetadata = []parseAWSCloudwatchMetadataTestData{
	{map[string]string{}, testAWSAuthentication, true, "Empty structures"},
	// properly formed cloudwatch query and awsRegion
	{map[string]string{
		"namespace":         "AWS/SQS",
		"dimensionName":     "QueueName",
		"dimensionValue":    "keda",
		"metricName":        "ApproximateNumberOfMessagesVisible",
		"targetMetricValue": "2",
		"minMetricValue":    "0",
		"awsRegion":         "eu-west-1"},
		testAWSAuthentication,
		false,
		"properly formed cloudwatch query and awsRegion"},
	// Properly formed cloudwatch query with optional parameters
	{map[string]string{
		"namespace":            "AWS/SQS",
		"dimensionName":        "QueueName",
		"dimensionValue":       "keda",
		"metricName":           "ApproximateNumberOfMessagesVisible",
		"targetMetricValue":    "2",
		"minMetricValue":       "0",
		"metricCollectionTime": "300",
		"metricStat":           "Average",
		"metricStatPeriod":     "300",
		"awsRegion":            "eu-west-1"},
		testAWSAuthentication, false,
		"Properly formed cloudwatch query with optional parameters"},
	// properly formed cloudwatch query but Region is empty
	{map[string]string{
		"namespace":         "AWS/SQS",
		"dimensionName":     "QueueName",
		"dimensionValue":    "keda",
		"metricName":        "ApproximateNumberOfMessagesVisible",
		"targetMetricValue": "2",
		"minMetricValue":    "0",
		"awsRegion":         ""},
		testAWSAuthentication,
		true,
		"properly formed cloudwatch query but Region is empty"},
	// Missing namespace
	{map[string]string{"dimensionName": "QueueName",
		"dimensionValue":    "keda",
		"metricName":        "ApproximateNumberOfMessagesVisible",
		"targetMetricValue": "2",
		"minMetricValue":    "0",
		"awsRegion":         "eu-west-1"},
		testAWSAuthentication,
		true,
		"Missing namespace"},
	// Missing dimensionName
	{map[string]string{
		"dimensionName":     "QueueName",
		"dimensionValue":    "keda",
		"metricName":        "ApproximateNumberOfMessagesVisible",
		"targetMetricValue": "2",
		"minMetricValue":    "0",
		"awsRegion":         "eu-west-1"},
		testAWSAuthentication,
		true,
		"Missing dimensionName"},
	// Missing dimensionValue
	{map[string]string{
		"namespace":         "AWS/SQS",
		"dimensionName":     "QueueName",
		"metricName":        "ApproximateNumberOfMessagesVisible",
		"targetMetricValue": "2",
		"minMetricValue":    "0",
		"awsRegion":         "eu-west-1"},
		testAWSAuthentication,
		true,
		"Missing dimensionValue"},
	// Missing metricName
	{map[string]string{
		"namespace":         "AWS/SQS",
		"dimensionName":     "QueueName",
		"dimensionValue":    "keda",
		"targetMetricValue": "2",
		"minMetricValue":    "0",
		"awsRegion":         "eu-west-1"},
		testAWSAuthentication,
		true,
		"Missing metricName"},
	// with "aws_credentials" from TriggerAuthentication
	{map[string]string{
		"namespace":            "AWS/SQS",
		"dimensionName":        "QueueName",
		"dimensionValue":       "keda",
		"metricName":           "ApproximateNumberOfMessagesVisible",
		"targetMetricValue":    "2",
		"minMetricValue":       "0",
		"metricCollectionTime": "300",
		"metricStat":           "Average",
		"metricStatPeriod":     "300",
		"awsRegion":            "eu-west-1"},
		map[string]string{
			"awsAccessKeyId":     testAWSCloudwatchAccessKeyID,
			"awsSecretAccessKey": testAWSCloudwatchSecretAccessKey,
		},
		false,
		"with AWS Credentials from TriggerAuthentication"},
	// with "aws_role" from TriggerAuthentication
	{map[string]string{
		"namespace":            "AWS/SQS",
		"dimensionName":        "QueueName",
		"dimensionValue":       "keda",
		"metricName":           "ApproximateNumberOfMessagesVisible",
		"targetMetricValue":    "2",
		"minMetricValue":       "0",
		"metricCollectionTime": "300",
		"metricStat":           "Average",
		"metricStatPeriod":     "300",
		"awsRegion":            "eu-west-1"},
		map[string]string{
			"awsRoleArn": testAWSCloudwatchRoleArn,
		},
		false,
		"with AWS Role from TriggerAuthentication"},
	{map[string]string{
		"namespace":            "AWS/SQS",
		"dimensionName":        "QueueName",
		"dimensionValue":       "keda",
		"metricName":           "ApproximateNumberOfMessagesVisible",
		"targetMetricValue":    "2",
		"minMetricValue":       "0",
		"metricCollectionTime": "300",
		"metricStat":           "Average",
		"metricStatPeriod":     "300",
		"awsRegion":            "eu-west-1",
		"identityOwner":        "operator"},
		map[string]string{},
		false,
		"with AWS Role assigned on KEDA operator itself"},
	{map[string]string{
		"namespace":            "AWS/SQS",
		"dimensionName":        "QueueName",
		"dimensionValue":       "keda",
		"metricName":           "ApproximateNumberOfMessagesVisible",
		"targetMetricValue":    "2",
		"minMetricValue":       "0",
		"metricCollectionTime": "a",
		"metricStat":           "Average",
		"metricStatPeriod":     "300",
		"awsRegion":            "eu-west-1",
		"identityOwner":        "operator"},
		map[string]string{},
		true,
		"if metricCollectionTime assigned with a string, need to be a number"},
	{map[string]string{
		"namespace":            "AWS/SQS",
		"dimensionName":        "QueueName",
		"dimensionValue":       "keda",
		"metricName":           "ApproximateNumberOfMessagesVisible",
		"targetMetricValue":    "2",
		"minMetricValue":       "0",
		"metricCollectionTime": "300",
		"metricStat":           "Average",
		"metricStatPeriod":     "a",
		"awsRegion":            "eu-west-1",
		"identityOwner":        "operator"},
		map[string]string{},
		true,
		"if metricStatPeriod assigned with a string, need to be a number"},
	{map[string]string{
		"namespace":         "AWS/SQS",
		"dimensionName":     "QueueName",
		"dimensionValue":    "keda",
		"metricName":        "ApproximateNumberOfMessagesVisible",
		"targetMetricValue": "2",
		"minMetricValue":    "0",
		"metricStat":        "Average",
		"metricStatPeriod":  "300",
		"awsRegion":         "eu-west-1"},
		testAWSAuthentication, false,
		"Missing metricCollectionTime not generate error because will get the default value"},
	{map[string]string{
		"namespace":            "AWS/SQS",
		"dimensionName":        "QueueName",
		"dimensionValue":       "keda",
		"metricName":           "ApproximateNumberOfMessagesVisible",
		"targetMetricValue":    "2",
		"minMetricValue":       "0",
		"metricCollectionTime": "300",
		"metricStatPeriod":     "300",
		"awsRegion":            "eu-west-1"},
		testAWSAuthentication, false,
		"Missing metricStat not generate error because will get the default value"},
	{map[string]string{
		"namespace":            "AWS/SQS",
		"dimensionName":        "QueueName",
		"dimensionValue":       "keda",
		"metricName":           "ApproximateNumberOfMessagesVisible",
		"targetMetricValue":    "2",
		"minMetricValue":       "0",
		"metricCollectionTime": "300",
		"metricStat":           "Average",
		"awsRegion":            "eu-west-1"},
		testAWSAuthentication, false,
		"Missing metricStatPeriod not generate error because will get the default value"},
	{map[string]string{
		"namespace":           "AWS/SQS",
		"dimensionName":       "QueueName",
		"dimensionValue":      "keda",
		"metricName":          "ApproximateNumberOfMessagesVisible",
		"targetMetricValue":   "2",
		"minMetricValue":      "0",
		"metricStat":          "Average",
		"metricUnit":          "Count",
		"metricEndTimeOffset": "60",
		"awsRegion":           "eu-west-1"},
		testAWSAuthentication, false,
		"set a supported metricUnit"},
	{map[string]string{
		"namespace":            "AWS/SQS",
		"dimensionName":        "QueueName",
		"dimensionValue":       "keda",
		"metricName":           "ApproximateNumberOfMessagesVisible",
		"targetMetricValue":    "2",
		"minMetricValue":       "0",
		"metricCollectionTime": "300",
		"metricStat":           "SomeStat",
		"awsRegion":            "eu-west-1"},
		testAWSAuthentication, true,
		"metricStat is not supported"},
	{map[string]string{
		"namespace":            "AWS/SQS",
		"dimensionName":        "QueueName",
		"dimensionValue":       "keda",
		"metricName":           "ApproximateNumberOfMessagesVisible",
		"targetMetricValue":    "2",
		"minMetricValue":       "0",
		"metricStatPeriod":     "300",
		"metricCollectionTime": "100",
		"metricStat":           "Average",
		"awsRegion":            "eu-west-1"},
		testAWSAuthentication, true,
		"metricCollectionTime smaller than metricStatPeriod"},
	{map[string]string{
		"namespace":         "AWS/SQS",
		"dimensionName":     "QueueName",
		"dimensionValue":    "keda",
		"metricName":        "ApproximateNumberOfMessagesVisible",
		"targetMetricValue": "2",
		"minMetricValue":    "0",
		"metricStatPeriod":  "250",
		"metricStat":        "Average",
		"awsRegion":         "eu-west-1"},
		testAWSAuthentication, true,
		"unsupported metricStatPeriod"},
	{map[string]string{
		"namespace":         "AWS/SQS",
		"dimensionName":     "QueueName",
		"dimensionValue":    "keda",
		"metricName":        "ApproximateNumberOfMessagesVisible",
		"targetMetricValue": "2",
		"minMetricValue":    "0",
		"metricStatPeriod":  "25",
		"metricStat":        "Average",
		"awsRegion":         "eu-west-1"},
		testAWSAuthentication, true,
		"unsupported metricStatPeriod"},
	{map[string]string{
		"namespace":         "AWS/SQS",
		"dimensionName":     "QueueName",
		"dimensionValue":    "keda",
		"metricName":        "ApproximateNumberOfMessagesVisible",
		"targetMetricValue": "2",
		"minMetricValue":    "0",
		"metricStatPeriod":  "25",
		"metricStat":        "Average",
		"metricUnit":        "Hour",
		"awsRegion":         "eu-west-1"},
		testAWSAuthentication, true,
		"unsupported metricUnit"},
}

var awsCloudwatchMetricIdentifiers = []awsCloudwatchMetricIdentifier{
	{&testAWSCloudwatchMetadata[1], 0, "s0-aws-cloudwatch-QueueName"},
	{&testAWSCloudwatchMetadata[1], 3, "s3-aws-cloudwatch-QueueName"},
}

var awsCloudwatchGetMetricTestData = []awsCloudwatchMetadata{
	{
		namespace:            "Custom",
		metricsName:          "HasData",
		dimensionName:        []string{"DIM"},
		dimensionValue:       []string{"DIM_VALUE"},
		targetMetricValue:    100,
		minMetricValue:       0,
		metricCollectionTime: 60,
		metricStat:           "Average",
		metricUnit:           "SampleCount",
		metricStatPeriod:     60,
		metricEndTimeOffset:  60,
		awsRegion:            "us-west-2",
		awsAuthorization:     awsAuthorizationMetadata{podIdentityOwner: false},
		scalerIndex:          0,
	},
	{
		namespace:            "Custom",
		metricsName:          "HasDataNoUnit",
		dimensionName:        []string{"DIM"},
		dimensionValue:       []string{"DIM_VALUE"},
		targetMetricValue:    100,
		minMetricValue:       0,
		metricCollectionTime: 60,
		metricStat:           "Average",
		metricUnit:           "",
		metricStatPeriod:     60,
		metricEndTimeOffset:  60,
		awsRegion:            "us-west-2",
		awsAuthorization:     awsAuthorizationMetadata{podIdentityOwner: false},
		scalerIndex:          0,
	},
	{
		namespace:            "Custom",
		metricsName:          testAWSCloudwatchErrorMetric,
		dimensionName:        []string{"DIM"},
		dimensionValue:       []string{"DIM_VALUE"},
		targetMetricValue:    100,
		minMetricValue:       0,
		metricCollectionTime: 60,
		metricStat:           "Average",
		metricUnit:           "",
		metricStatPeriod:     60,
		metricEndTimeOffset:  60,
		awsRegion:            "us-west-2",
		awsAuthorization:     awsAuthorizationMetadata{podIdentityOwner: false},
		scalerIndex:          0,
	},
	{
		namespace:            "Custom",
		metricsName:          testAWSCloudwatchNoValueMetric,
		dimensionName:        []string{"DIM"},
		dimensionValue:       []string{"DIM_VALUE"},
		targetMetricValue:    100,
		minMetricValue:       0,
		metricCollectionTime: 60,
		metricStat:           "Average",
		metricUnit:           "",
		metricStatPeriod:     60,
		metricEndTimeOffset:  60,
		awsRegion:            "us-west-2",
		awsAuthorization:     awsAuthorizationMetadata{podIdentityOwner: false},
		scalerIndex:          0,
	},
}

type mockCloudwatch struct {
	cloudwatchiface.CloudWatchAPI
}

func (m *mockCloudwatch) GetMetricData(input *cloudwatch.GetMetricDataInput) (*cloudwatch.GetMetricDataOutput, error) {
	switch *input.MetricDataQueries[0].MetricStat.Metric.MetricName {
	case testAWSCloudwatchErrorMetric:
		return nil, errors.New("error")
	case testAWSCloudwatchNoValueMetric:
		return &cloudwatch.GetMetricDataOutput{
			MetricDataResults: []*cloudwatch.MetricDataResult{},
		}, nil
	}
	return &cloudwatch.GetMetricDataOutput{
		MetricDataResults: []*cloudwatch.MetricDataResult{
			{
				Values: []*float64{aws.Float64(10)},
			},
		},
	}, nil
}

func TestCloudwatchParseMetadata(t *testing.T) {
	for _, testData := range testAWSCloudwatchMetadata {
		_, err := parseAwsCloudwatchMetadata(&ScalerConfig{TriggerMetadata: testData.metadata, ResolvedEnv: testAWSCloudwatchResolvedEnv, AuthParams: testData.authParams})
		if err != nil && !testData.isError {
			t.Errorf("%s: Expected success but got error %s", testData.comment, err)
		}
		if testData.isError && err == nil {
			t.Errorf("%s: Expected error but got success", testData.comment)
		}
	}
}

func TestAWSCloudwatchGetMetricSpecForScaling(t *testing.T) {
	for _, testData := range awsCloudwatchMetricIdentifiers {
		ctx := context.Background()
		meta, err := parseAwsCloudwatchMetadata(&ScalerConfig{TriggerMetadata: testData.metadataTestData.metadata, ResolvedEnv: testAWSCloudwatchResolvedEnv, AuthParams: testData.metadataTestData.authParams, ScalerIndex: testData.scalerIndex})
		if err != nil {
			t.Fatal("Could not parse metadata:", err)
		}
		mockAWSCloudwatchScaler := awsCloudwatchScaler{meta, &mockCloudwatch{}}

		metricSpec := mockAWSCloudwatchScaler.GetMetricSpecForScaling(ctx)
		metricName := metricSpec[0].External.Metric.Name
		if metricName != testData.name {
			t.Error("Wrong External metric source name:", metricName)
		}
	}
}

func TestAWSCloudwatchScalerGetMetrics(t *testing.T) {
	var selector labels.Selector
	for _, meta := range awsCloudwatchGetMetricTestData {
		mockAWSCloudwatchScaler := awsCloudwatchScaler{&meta, &mockCloudwatch{}}
		value, err := mockAWSCloudwatchScaler.GetMetrics(context.Background(), meta.metricsName, selector)
		switch meta.metricsName {
		case testAWSCloudwatchErrorMetric:
			assert.Error(t, err, "expect error because of cloudwatch api error")
		case testAWSCloudwatchNoValueMetric:
			assert.NoError(t, err, "dont expect error when returning empty metric list from cloudwatch")
		default:
			assert.EqualValues(t, int64(10.0), value[0].Value.Value())
		}
	}
}

type computeQueryWindowTestArgs struct {
	name                    string
	current                 string
	metricPeriodSec         int64
	metricEndTimeOffsetSec  int64
	metricCollectionTimeSec int64
	expectedStartTime       string
	expectedEndTime         string
}

var awsCloudwatchComputeQueryWindowTestData = []computeQueryWindowTestArgs{
	{
		name:                    "normal",
		current:                 "2021-11-07T15:04:05.999Z",
		metricPeriodSec:         60,
		metricEndTimeOffsetSec:  0,
		metricCollectionTimeSec: 60,
		expectedStartTime:       "2021-11-07T15:03:00Z",
		expectedEndTime:         "2021-11-07T15:04:00Z",
	},
	{
		name:                    "normal with offset",
		current:                 "2021-11-07T15:04:05.999Z",
		metricPeriodSec:         60,
		metricEndTimeOffsetSec:  30,
		metricCollectionTimeSec: 60,
		expectedStartTime:       "2021-11-07T15:02:00Z",
		expectedEndTime:         "2021-11-07T15:03:00Z",
	},
}

func TestComputeQueryWindow(t *testing.T) {
	for _, testData := range awsCloudwatchComputeQueryWindowTestData {
		current, err := time.Parse(time.RFC3339Nano, testData.current)
		if err != nil {
			t.Errorf("unexpected input datetime format: %v", err)
		}
		startTime, endTime := computeQueryWindow(current, testData.metricPeriodSec, testData.metricEndTimeOffsetSec, testData.metricCollectionTimeSec)
		assert.Equal(t, testData.expectedStartTime, startTime.UTC().Format(time.RFC3339Nano), "unexpected startTime", "name", testData.name)
		assert.Equal(t, testData.expectedEndTime, endTime.UTC().Format(time.RFC3339Nano), "unexpected endTime", "name", testData.name)
	}
}
