// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package jmx

const (
	annotationPrefix = "cloudwatch.aws.amazon.com/inject-jmx-"
)

const (
	EnvTargetSystem = "OTEL_JMX_TARGET_SYSTEM"

	TargetJVM           = "jvm"
	TargetTomcat        = "tomcat"
	TargetKafka         = "kafka"
	TargetKafkaConsumer = "kafka-consumer"
	TargetKafkaProducer = "kafka-producer"
)

var SupportedTargets = []string{TargetJVM, TargetTomcat, TargetKafka, TargetKafkaConsumer, TargetKafkaProducer}

func AnnotationKey(target string) string {
	return annotationPrefix + target
}
