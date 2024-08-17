// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package rbac

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/authorization/v1"
)

// WarningsGroupedByResource is a helper to take the missing permissions and format them as warnings.
func WarningsGroupedByResource(reviews []*v1.SubjectAccessReview) []string {
	fullResourceToVerbs := make(map[string][]string)
	for _, review := range reviews {
		if review.Spec.ResourceAttributes != nil {
			key := fmt.Sprintf("%s/%s", review.Spec.ResourceAttributes.Group, review.Spec.ResourceAttributes.Resource)
			if len(review.Spec.ResourceAttributes.Group) == 0 {
				key = review.Spec.ResourceAttributes.Resource
			}
			fullResourceToVerbs[key] = append(fullResourceToVerbs[key], review.Spec.ResourceAttributes.Verb)
		} else if review.Spec.NonResourceAttributes != nil {
			key := fmt.Sprintf("nonResourceURL: %s", review.Spec.NonResourceAttributes.Path)
			fullResourceToVerbs[key] = append(fullResourceToVerbs[key], review.Spec.NonResourceAttributes.Verb)
		}
	}
	var warnings []string
	for fullResource, verbs := range fullResourceToVerbs {
		warnings = append(warnings, fmt.Sprintf("missing the following rules for %s: [%s]", fullResource, strings.Join(verbs, ",")))
	}
	return warnings
}
