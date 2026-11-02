package main

import (
	"encoding/json"
	"fmt"
	"os"

	"mdiff/ledger"
)

func main() {
	pairs := [][2]string{
		{"Kadang aku butuh waktu sendiri.", "Kamu kadang butuh waktu sendiri."},
		{"Kadang aku butuh waktu sendiri.", "Kamu kadang perlu waktu sendiri."},
		{"Kadang aku butuh waktu sendiri, tapi aku tetap senang bertemu teman-temanku.", "Kadang aku butuh waktu sendiri, tapi aku tetap senang bertemu teman."},
		{"Kadang aku butuh waktu sendiri, tapi aku tetap senang bertemu teman-temanku.", "Kamu selalu butuh waktu sendiri dan menghindari teman."},
		{"Kadang aku butuh waktu sendiri, tapi aku tetap senang bertemu teman-temanku.", "Nadia selalu menghindari teman-temannya dan tidak suka bergaul."},
		{"Kadang aku butuh waktu sendiri.", "Kamu selalu butuh waktu sendiri."},
		{"Kadang aku butuh waktu sendiri, tapi aku tetap senang bertemu teman-temanku.", "Kamu selalu butuh waktu sendiri."},
		{"Kadang aku butuh waktu sendiri, tapi aku tetap senang bertemu teman-temanku.", "Kamu selalu butuh waktu sendiri dan menghindari teman."},
		{"Kadang aku butuh waktu sendiri, tapi aku tetap senang bertemu teman-temanku.", "Kamu selalu butuh waktu sendiri, tapi kamu tetap senang bertemu teman-temanmu."},
		{"Kadang aku butuh waktu sendiri, tapi aku tetap senang bertemu teman-temanku.", "Kadang kamu butuh waktu sendiri, tapi kamu tetap senang bertemu teman-temanmu."},
		{"Kadang aku butuh waktu sendiri, tapi aku tetap senang bertemu teman-temanku.", "Kadang aku perlu waktu tenang setelah kuliah, tetapi aku tetap menikmati waktu bersama teman. Aku belum tahu penyebab rasa lelahku."},
	}
	enc := json.NewEncoder(os.Stdout)
	for _, p := range pairs {
		fmt.Printf("%q\n  -> %q\n  ", p[0], p[1])
		enc.Encode(ledger.CompareMeaning(p[0], p[1]))
	}
}
