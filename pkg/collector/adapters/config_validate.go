// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"github.com/go-logr/logr"
)

// Following Otel Doc: Configuring a receiver does not enable it. The receivers are enabled via pipelines within the service section.
// GetEnabledReceivers returns all enabled receivers as a true flag set. If it can't find any receiver, it will return a nil interface.
func GetEnabledReceivers(_ logr.Logger, config map[interface{}]interface{}) map[interface{}]bool {
	cfgReceivers, ok := config["receivers"]
	if !ok {
		return nil
	}
	receivers, ok := cfgReceivers.(map[interface{}]interface{})
	if !ok {
		return nil
	}
	availableReceivers := map[interface{}]bool{}

	for recvID := range receivers {

		//Safe Cast
		receiverID, withReceiver := recvID.(string)
		if !withReceiver {
			return nil
		}
		//Getting all receivers present in the receivers section and setting them to false.
		availableReceivers[receiverID] = false
	}

	cfgService, withService := config["service"].(map[interface{}]interface{})
	if !withService {
		return nil
	}

	pipeline, withPipeline := cfgService["pipelines"].(map[interface{}]interface{})
	if !withPipeline {
		return nil
	}
	availablePipelines := map[string]bool{}

	for pipID := range pipeline {
		//Safe Cast
		pipelineID, existsPipeline := pipID.(string)
		if !existsPipeline {
			return nil
		}
		//Getting all the available pipelines.
		availablePipelines[pipelineID] = true
	}

	if len(pipeline) > 0 {
		for pipelineID, pipelineCfg := range pipeline {
			//Safe Cast
			pipelineV, withPipelineCfg := pipelineID.(string)
			if !withPipelineCfg {
				continue
			}
			//Condition will get information if there are multiple configured pipelines.
			if len(pipelineV) > 0 {
				pipelineDesc, ok := pipelineCfg.(map[interface{}]interface{})
				if !ok {
					return nil
				}
				for pipSpecID, pipSpecCfg := range pipelineDesc {
					if pipSpecID.(string) == "receivers" {
						receiversList, ok := pipSpecCfg.([]interface{})
						if !ok {
							continue
						}
						// If receiversList is empty means that we haven't any enabled Receiver.
						if len(receiversList) == 0 {
							availableReceivers = nil
						} else {
							// All enabled receivers will be set as true
							for _, recKey := range receiversList {
								//Safe Cast
								receiverKey, ok := recKey.(string)
								if !ok {
									return nil
								}
								availableReceivers[receiverKey] = true
							}
						}
						//Removing all non-enabled receivers
						for recID, recKey := range availableReceivers {
							if !(recKey) {
								delete(availableReceivers, recID)
							}
						}
					}
				}
			}
		}
	}
	return availableReceivers
}
