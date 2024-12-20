package bootstrapnode

const (
	// BootstrapCertificateIssuer is the public key of the bootstrap Certificate Issuer.
	BootstrapCertificateIssuer = "0319f86fa35ef9bcfdca01de56cf65833a2af81c299a40dff88fe7874cb1423d02" //

	BootStrapNodeId = 10000104
)

func VerifyCertificate(certificate []byte) bool {
	return true
}

func IsBootStrapNode(validatorId uint64, publicKey []byte) bool {
	if validatorId == BootStrapNodeId {
		return true
	}

	return false
}
