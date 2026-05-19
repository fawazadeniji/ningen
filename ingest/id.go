package ingest

import "github.com/google/uuid"

// itemNS is a fixed UUID v4 namespace for deterministic item ID generation.
// Changing this value would invalidate all existing IDs — never change it.
var itemNS = uuid.MustParse("3f7a9c2e-4b81-4d6f-9e32-a1b5c8d70f4e")

// deterministicID returns a UUID v5 derived from domain + text.
// The same domain+text always produces the same ID, enabling dedup on reingest.
func deterministicID(domain, text string) string {
	return uuid.NewSHA1(itemNS, []byte(domain+"\x00"+text)).String()
}
