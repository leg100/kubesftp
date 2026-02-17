package internal

type algorithm string

const (
	ed25519Algorithm algorithm = "ed25519"
	ecdsaAlgorithm   algorithm = "ecdsa"
	rsaAlgorithm     algorithm = "rsa"
)
