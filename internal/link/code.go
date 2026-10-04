package link

import "crypto/rand"

const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

func generateCode(n int) string {
	buf := make([]byte, n)
	rand.Read(buf)
	for i, b := range buf {
		buf[i] = alphabet[b&63]
	}
	return string(buf)
}
