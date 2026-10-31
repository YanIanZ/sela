//go:build probe

package safety

import ("fmt";"testing")

func TestNorm(t *testing.T) {
	for _, s := range []string{"Aku mau BUNUH dia!!", "gw mo nusuk org itu", "Aku nggak mau bunuh dia", "I'm going to k1ll him", "Suamiku mukulin aku tiap malem", "aku g4k aman di rmh"} {
		fmt.Printf("%q -> %q\n", s, Normalize(s))
	}
}
