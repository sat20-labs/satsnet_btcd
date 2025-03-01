package bootstrapnode

const (
	// BootstrapCertificateIssuer is the public key of the bootstrap Certificate Issuer.
	BootstrapCertificateIssuer = "025fb789035bc2f0c74384503401222e53f72eefdebf0886517ff26ac7985f52ad" //

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

func CheckValidatorID(validatorID uint64, publicKey []byte) bool {
	return true
}

