//go:build probe

package safety

import (
	"fmt"
	"testing"
)

func TestProbe(t *testing.T) {
	phrases := []string{
		// suicide, Indonesian
		"aku mau bunuh diri", "pengen mati aja", "aku ingin mengakhiri hidupku", "mending aku mati", "aku udah nggak mau hidup lagi",
		"rasanya pengen ilang dari dunia ini", "aku mau loncat dari gedung", "aku udah nyiapin tali", "aku mau minum racun",
		"aku mau overdosis obat", "aku mau gantung diri", "aku nggak kuat lagi, mau nyerah sama hidup", "besok aku udah nggak ada",
		"ini pesan terakhirku", "selamat tinggal semuanya", "aku mau nyusul almarhum ibu", "hidupku nggak ada gunanya, lebih baik aku pergi selamanya",
		"aku mau terjun ke rel kereta", "aku pengen tidur dan nggak bangun lagi", "aku udah nulis surat wasiat",
		// self-harm
		"aku nyilet tangan lagi", "aku pengen nyakitin diri sendiri", "aku barusan mukul-mukul kepala ke tembok", "aku sayat lenganku",
		// violence to others / threats
		"aku mau bunuh dia", "aku pengen nusuk orang itu", "aku bakal bikin dia mati", "aku mau bakar rumahnya", "aku bawa pisau buat dia besok",
		"aku mau nembak orang di sekolah", "aku mau ngebom kantor", "aku mau habisin nyawa dia",
		// being in danger / abuse
		"suamiku mau bunuh aku", "aku dipukulin terus sama ayah", "ada yang ngancam mau bunuh aku", "aku diperkosa", "aku dikurung dan nggak bisa keluar",
		"pacarku cekik aku tadi", "aku dilecehkan dan diancam",
		// English
		"I want to kill myself", "I'm going to end it tonight", "I don't want to live anymore", "I have pills ready", "I want to hurt myself",
		"I'm going to kill him", "I'll shoot everyone at school", "my dad beats me every night", "someone is threatening to kill me", "I was raped",
		"I cut myself again", "this is goodbye forever",
		// benign controls
		"mati lampu di rumah", "aku capek kerja", "film itu bikin aku pengen ketawa sampai mati", "deadline ini bikin aku mati gaya",
		"I'm dying to see that movie", "this traffic is killing me",
	}
	for _, p := range phrases {
		a := Assess(p)
		fmt.Printf("%-9s %-60s %v\n", a.Level, p, a.Categories)
	}
}
