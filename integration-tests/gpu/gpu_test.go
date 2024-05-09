// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build !windows

package emf

import (
	"fmt"
	"log"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/aws/amazon-cloudwatch-agent-test/environment"
	"github.com/aws/amazon-cloudwatch-agent-test/environment/computetype"
	"github.com/aws/amazon-cloudwatch-agent-test/test/metric/dimension"
	"github.com/aws/amazon-cloudwatch-agent-test/test/status"
	"github.com/aws/amazon-cloudwatch-agent-test/test/test_runner"
)

type GPUTestSuite struct {
	suite.Suite
	test_runner.TestSuite
}

func (suite *GPUTestSuite) SetupSuite() {
	fmt.Println(">>>> Starting GPU Container Insights TestSuite")
}

func (suite *GPUTestSuite) TearDownSuite() {
	suite.Result.Print()
	fmt.Println(">>>> Finished GPU Container Insights TestSuite")
}

func init() {
	environment.RegisterEnvironmentMetaDataFlags()
}

var (
	eksTestRunners []*test_runner.EKSTestRunner
)

func getEksTestRunners(env *environment.MetaData) []*test_runner.EKSTestRunner {
	if eksTestRunners == nil {
		factory := dimension.GetDimensionFactory(*env)

		eksTestRunners = []*test_runner.EKSTestRunner{
			{
				Runner: &NvidiaTestRunner{test_runner.BaseTestRunner{DimensionFactory: factory}, "EKS_GPU_NVIDIA", env},
				Env:    *env,
			},
		}
	}
	return eksTestRunners
}

func (suite *GPUTestSuite) TestAllInSuite() {
	env := environment.GetEnvironmentMetaData()
	switch env.ComputeType {
	case computetype.EKS:
		log.Println("Environment compute type is EKS")
		for _, testRunner := range getEksTestRunners(env) {
			testRunner.Run(suite, env)
		}
	default:
		return
	}

	suite.Assert().Equal(status.SUCCESSFUL, suite.Result.GetStatus(), "GPU Container Test Suite Failed")
}

func (suite *GPUTestSuite) AddToSuiteResult(r status.TestGroupResult) {
	suite.Result.TestGroupResults = append(suite.Result.TestGroupResults, r)
}

func TestGPUSuite(t *testing.T) {
	suite.Run(t, new(GPUTestSuite))
}
