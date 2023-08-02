package v1alpha1

type (
	// IngressType represents how a collector should be exposed (ingress vs route).
	// +kubebuilder:validation:Enum=ingress;route
	IngressType string
)

const (
	// IngressTypeNginx specifies that an ingress entry should be created.
	IngressTypeNginx IngressType = "ingress"
	// IngressTypeOpenshiftRoute specifies that an route entry should be created.
	IngressTypeRoute IngressType = "route"
)

type (
	// TLSRouteTerminationType is used to indicate which tls settings should be used.
	// +kubebuilder:validation:Enum=insecure;edge;passthrough;reencrypt
	TLSRouteTerminationType string
)

const (
	// TLSRouteTerminationTypeInsecure indicates that insecure connections are allowed.
	TLSRouteTerminationTypeInsecure TLSRouteTerminationType = "insecure"
	// TLSRouteTerminationTypeEdge indicates that encryption should be terminated
	// at the edge router.
	TLSRouteTerminationTypeEdge TLSRouteTerminationType = "edge"
	// TLSTerminationPassthrough indicates that the destination service is
	// responsible for decrypting traffic.
	TLSRouteTerminationTypePassthrough TLSRouteTerminationType = "passthrough"
	// TLSTerminationReencrypt indicates that traffic will be decrypted on the edge
	// and re-encrypt using a new certificate.
	TLSRouteTerminationTypeReencrypt TLSRouteTerminationType = "reencrypt"
)
