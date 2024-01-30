// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"fmt"

	routev1 "github.com/openshift/api/route/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/aws/amazon-cloudwatch-agent-operator/apis/v1alpha1"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/manifests"
	"github.com/aws/amazon-cloudwatch-agent-operator/internal/naming"
)

func Routes(params manifests.Params) ([]*routev1.Route, error) {
	if params.OtelCol.Spec.Ingress.Type != v1alpha1.IngressTypeRoute {
		return nil, nil
	}

	if params.OtelCol.Spec.Mode == v1alpha1.ModeSidecar {
		params.Log.V(3).Info("ingress settings are not supported in sidecar mode")
		return nil, nil
	}

	var tlsCfg *routev1.TLSConfig
	switch params.OtelCol.Spec.Ingress.Route.Termination {
	case v1alpha1.TLSRouteTerminationTypeInsecure:
		// NOTE: insecure, no tls cfg.
	case v1alpha1.TLSRouteTerminationTypeEdge:
		tlsCfg = &routev1.TLSConfig{Termination: routev1.TLSTerminationEdge}
	case v1alpha1.TLSRouteTerminationTypePassthrough:
		tlsCfg = &routev1.TLSConfig{Termination: routev1.TLSTerminationPassthrough}
	case v1alpha1.TLSRouteTerminationTypeReencrypt:
		tlsCfg = &routev1.TLSConfig{Termination: routev1.TLSTerminationReencrypt}
	default: // NOTE: if unsupported, end here.
		return nil, nil
	}

	ports, err := servicePortsFromCfg(params.Log, params.OtelCol)

	// if we have no ports, we don't need a ingress entry
	if len(ports) == 0 || err != nil {
		params.Log.V(1).Info(
			"the instance's configuration didn't yield any ports to open, skipping ingress",
			"instance.name", params.OtelCol.Name,
			"instance.namespace", params.OtelCol.Namespace,
		)
		return nil, err
	}

	routes := make([]*routev1.Route, len(ports))
	for i, p := range ports {
		portName := naming.PortName(p.Name, p.Port)
		host := ""
		if params.OtelCol.Spec.Ingress.Hostname != "" {
			host = fmt.Sprintf("%s.%s", portName, params.OtelCol.Spec.Ingress.Hostname)
		}

		routes[i] = &routev1.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name:        naming.Route(params.OtelCol.Name, p.Name),
				Namespace:   params.OtelCol.Namespace,
				Annotations: params.OtelCol.Spec.Ingress.Annotations,
				Labels: map[string]string{
					"app.kubernetes.io/name":       naming.Route(params.OtelCol.Name, p.Name),
					"app.kubernetes.io/instance":   fmt.Sprintf("%s.%s", params.OtelCol.Namespace, params.OtelCol.Name),
					"app.kubernetes.io/managed-by": "amazon-cloudwatch-agent-operator",
					"app.kubernetes.io/component":  "amazon-cloudwatch-agent",
				},
			},
			Spec: routev1.RouteSpec{
				Host: host,
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: naming.Service(params.OtelCol.Name),
				},
				Port: &routev1.RoutePort{
					TargetPort: intstr.FromString(portName),
				},
				WildcardPolicy: routev1.WildcardPolicyNone,
				TLS:            tlsCfg,
			},
		}
	}
	return routes, nil
}
