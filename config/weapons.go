package config

// Combat values

// Max Damage
var MaxWeaponDamage = map[int]int{
	1:  15,
	2:  20,
	3:  25,
	4:  30,
	5:  35,
	6:  40,
	7:  45,
	8:  50,
	9:  55,
	10: 60,
	11: 65,
	12: 70,
	13: 75,
	14: 80,
	15: 85,
	16: 90,
	17: 95,
	18: 100,
	19: 105,
	20: 110,
	21: 115,
	22: 120,
	23: 125,
	24: 130,
	25: 135,
	26: 140,
}

// Quick Function to check if character can wield
func CanWield(tier int, class int, max int) bool {
	if class == 0 {
		tier += 1
	}
	if max < MaxWeaponDamage[tier] {
		return true
	}
	return false
}

// Exp to level weapon classes
var WeaponExpLevels = map[int]int{
	0:  0,
	1:  1500,
	2:  3000,
	3:  30000,
	4:  300000,
	5:  750000,
	6:  1500000,
	7:  2250000,
	8:  3000000,
	9:  7500000,
	10: 15000000,
	11: 45000000,
}

var SkillAdvancement = map[int]float32{
	0: 1,
	1: .7,
	2: .7,
	3: .7,
	6: .7,
	8: .5,
	7: .5,
	5: .4,
	4: .4,
}

var WeaponTitles = []string{
	"Unskilled",
	"Basic",
	"Skilled",
	"Experienced",
	"Refined",
	"Ace",
	"Adept",
	"Expert",
	"Specialist",
	"Master",
	"Grandmaster",
}

func WeaponExpTitle(exp int, class int) string {
	switch {
	case exp > WeaponExpLevels[0] && exp < WeaponExpLevels[1]:
		return WeaponTitles[0]
	case exp > WeaponExpLevels[1] && exp < WeaponExpLevels[2]:
		return WeaponTitles[1]
	case exp > WeaponExpLevels[2] && exp < WeaponExpLevels[3]:
		return WeaponTitles[2]
	case exp > WeaponExpLevels[3] && exp < WeaponExpLevels[4]:
		return WeaponTitles[3]
	case exp > WeaponExpLevels[4] && exp < WeaponExpLevels[5]:
		return WeaponTitles[4]
	case exp > WeaponExpLevels[5] && exp < WeaponExpLevels[6]:
		return WeaponTitles[5]
	case exp > WeaponExpLevels[6] && exp < WeaponExpLevels[7]:
		return WeaponTitles[6]
	case exp > WeaponExpLevels[7] && exp < WeaponExpLevels[8]:
		return WeaponTitles[7]
	case exp > WeaponExpLevels[8] && exp < WeaponExpLevels[9]:
		return WeaponTitles[8]
	case exp > WeaponExpLevels[9] && exp < WeaponExpLevels[10]:
		return WeaponTitles[9]
	case exp >= WeaponExpLevels[10]:
		if class == 0 {
			return WeaponTitles[10]
		} else {
			return WeaponTitles[9]
		}
	default:
		return WeaponTitles[0]
	}
}

func WeaponLevel(exp int, class int) int {
	switch {
	case exp > WeaponExpLevels[0] && exp < WeaponExpLevels[1]:
		return 0
	case exp > WeaponExpLevels[1] && exp < WeaponExpLevels[2]:
		return 1
	case exp > WeaponExpLevels[2] && exp < WeaponExpLevels[3]:
		return 2
	case exp > WeaponExpLevels[3] && exp < WeaponExpLevels[4]:
		return 3
	case exp > WeaponExpLevels[4] && exp < WeaponExpLevels[5]:
		return 4
	case exp > WeaponExpLevels[5] && exp < WeaponExpLevels[6]:
		return 5
	case exp > WeaponExpLevels[6] && exp < WeaponExpLevels[7]:
		return 6
	case exp > WeaponExpLevels[7] && exp < WeaponExpLevels[8]:
		return 7
	case exp > WeaponExpLevels[8] && exp < WeaponExpLevels[9]:
		return 8
	case exp > WeaponExpLevels[9] && exp < WeaponExpLevels[10]:
		return 9
	case exp > WeaponExpLevels[10] && exp < WeaponExpLevels[11]:
		return 10
	case exp >= WeaponExpLevels[11]:
		if class == 0 {
			return 11
		} else {
			return 10
		}
	default:
		return 0
	}
}

func WeaponExpNext(exp int, class int) int {
	switch {
	case exp > WeaponExpLevels[0] && exp < WeaponExpLevels[1]:
		return WeaponExpLevels[1]
	case exp > WeaponExpLevels[1] && exp < WeaponExpLevels[2]:
		return WeaponExpLevels[2]
	case exp > WeaponExpLevels[2] && exp < WeaponExpLevels[3]:
		return WeaponExpLevels[3]
	case exp > WeaponExpLevels[3] && exp < WeaponExpLevels[4]:
		return WeaponExpLevels[4]
	case exp > WeaponExpLevels[4] && exp < WeaponExpLevels[5]:
		return WeaponExpLevels[5]
	case exp > WeaponExpLevels[5] && exp < WeaponExpLevels[6]:
		return WeaponExpLevels[6]
	case exp > WeaponExpLevels[6] && exp < WeaponExpLevels[7]:
		return WeaponExpLevels[7]
	case exp > WeaponExpLevels[7] && exp < WeaponExpLevels[8]:
		return WeaponExpLevels[8]
	case exp > WeaponExpLevels[8] && exp < WeaponExpLevels[9]:
		return WeaponExpLevels[9]
	case exp > WeaponExpLevels[9] && exp < WeaponExpLevels[10]:
		if class == 0 {
			return WeaponExpLevels[10]
		} else {
			return 0
		}
	case exp >= WeaponExpLevels[10]:
		return 0
	default:
		return WeaponExpLevels[1]
	}
}

func WeaponMissChance(exp int, class int) int {
	switch {
	case exp > WeaponExpLevels[0] && exp < WeaponExpLevels[1]:
		return 45
	case exp > WeaponExpLevels[1] && exp < WeaponExpLevels[2]:
		return 35
	case exp > WeaponExpLevels[2] && exp < WeaponExpLevels[3]:
		return 25
	case exp > WeaponExpLevels[3] && exp < WeaponExpLevels[4]:
		return 15
	case exp > WeaponExpLevels[4] && exp < WeaponExpLevels[5]:
		return 5
	case exp > WeaponExpLevels[5]:
		return 0
	default:
		return 50
	}
}
