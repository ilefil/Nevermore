package objects

import (
	"github.com/ArcCS/Nevermore/config"
	"github.com/ArcCS/Nevermore/data"
	"github.com/ArcCS/Nevermore/permissions"
	"github.com/ArcCS/Nevermore/text"
	"github.com/ArcCS/Nevermore/utils"
	"github.com/jinzhu/copier"
	"log"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// Mob implements a control object for mobs interacting with players and each other
type Mob struct {
	Object
	MobId     int
	Inventory *ItemInventory
	ItemList  map[int]int
	Flags     map[string]bool
	Effects   map[string]*Effect
	Hooks     map[string]map[string]*Hook

	// ParentId is the room id for the room
	ParentId   int
	Gold       int
	Experience int
	Level      int

	Stam Meter
	Mana Meter

	// Attributes
	Str   Meter
	Dex   Meter
	Con   Meter
	Int   Meter
	Pie   Meter
	Armor int

	// Dice
	NumDice   int
	SidesDice int
	PlusDice  int

	// Magic
	ChanceCast      int
	Spells          []string
	WaterResistance int
	AirResistance   int
	FireResistance  int
	EarthResistance int
	BreathWeapon    string

	//Threat table attacker -> damage
	TotalThreatDamage int
	ThreatTable       map[string]int
	CurrentTarget     string

	NumWander  int
	TicksAlive int
	WimpyValue int

	MobTickerUnload chan bool
	MobCommands     chan string
	MobTicker       *time.Ticker
	// An int to hold a stun time.
	MobStunned int
	IsActive   bool
}

// Pop the mob data
func LoadMob(mobData map[string]interface{}) (*Mob, bool) {
	description := ""
	var ok bool
	if description, ok = mobData["description"].(string); !ok {
		description = "A mob...  yup"
	}
	newMob := &Mob{
		Object{
			Name:        mobData["name"].(string),
			Description: description,
			Placement:   5,
		},
		int(mobData["mob_id"].(int64)),
		NewItemInventory(),
		make(map[int]int),
		make(map[string]bool),
		make(map[string]*Effect),
		map[string]map[string]*Hook{
			"act":      make(map[string]*Hook),
			"combat":   make(map[string]*Hook),
			"peek":     make(map[string]*Hook),
			"gridmove": make(map[string]*Hook),
			"move":     make(map[string]*Hook),
			"say":      make(map[string]*Hook),
			"attacked": make(map[string]*Hook),
		},
		-1,
		int(mobData["gold"].(int64)),
		int(mobData["experience"].(int64)),
		int(mobData["level"].(int64)),
		Meter{int(mobData["hpmax"].(int64)), int(mobData["hpcur"].(int64))},
		Meter{int(mobData["mpmax"].(int64)), int(mobData["mpcur"].(int64))},
		Meter{40, int(mobData["strength"].(int64))},
		Meter{40, int(mobData["dexterity"].(int64))},
		Meter{40, int(mobData["constitution"].(int64))},
		Meter{40, int(mobData["intelligence"].(int64))},
		Meter{40, int(mobData["piety"].(int64))},
		int(mobData["armor"].(int64)),
		int(mobData["ndice"].(int64)),
		int(mobData["sdice"].(int64)),
		int(mobData["pdice"].(int64)),
		int(mobData["casting_probability"].(int64)),
		[]string{},
		int(mobData["water_resistance"].(int64)),
		int(mobData["air_resistance"].(int64)),
		int(mobData["fire_resistance"].(int64)),
		int(mobData["earth_resistance"].(int64)),
		mobData["breathes"].(string),
		0,
		nil,
		"",
		int(mobData["numwander"].(int64)),
		0,
		int(mobData["wimpyvalue"].(int64)),
		nil,
		nil,
		nil,
		0,
		false,
	}

	for _, spellN := range strings.Split(mobData["spells"].(string), ",") {
		if spellN != "" {
			newMob.Spells = append(newMob.Spells, spellN)
		}
	}

	for _, drop := range mobData["drops"].([]interface{}) {
		if drop != nil {
			dropData := drop.(map[string]interface{})
			if dropData["chance"] != nil {
				newMob.ItemList[int(dropData["item_id"].(int64))] = int(dropData["chance"].(int64))
			}
		}
	}

	for k, v := range mobData["flags"].(map[string]interface{}) {
		if v == nil {
			newMob.Flags[k] = false
		} else {
			newMob.Flags[k] = int(v.(int64)) != 0
		}
	}
	return newMob, true
}

func (m *Mob) StartTicking() {
	if m.IsActive {
		log.Println("mob active, not restarting")
		return
	}
	log.Println("not active starting ticking" + m.Name)
	m.IsActive = true
	m.CalculateInventory()
	m.ThreatTable = make(map[string]int)
	m.MobTickerUnload = make(chan bool)
	m.MobCommands = make(chan string)
	tickModifier := 0
	if fastMoving, ok := m.Flags["fast_moving"]; ok {
		if fastMoving {
			tickModifier = 2
		}
	}
	m.MobTicker = time.NewTicker(time.Duration(8-tickModifier) * time.Second)
	for _, spell := range m.Spells {
		if utils.StringIn(spell, MobSupportSpells) {
			Cast(m, m, spell, 0)
		}
	}
	go func() {
		for {
			select {
			case msg := <-m.MobCommands:
				// This function call will immediately call a command off the stack and run it, ideally to decouple state
				var params = strings.Split(msg, " ")
				go m.ProcessCommand(params[0], params[1:])
			case <-m.MobTickerUnload:
				log.Println("Unloading Mob Ticker for ", m.Name)
				m.MobTicker.Stop()
				m.IsActive = false
				return
			case <-m.MobTicker.C:
				log.Println("Locking Room for tick ", m.Name)
				Rooms[m.ParentId].Lock()
				m.Tick()
				log.Println("Unlocking Room post tick ", m.Name)
				Rooms[m.ParentId].Unlock()
			}
		}
	}()
}

func (m *Mob) GetSpellMultiplier() float32 {
	return 1
}

func (m *Mob) CheckThreatTable(charName string) bool {
	if _, ok := m.ThreatTable[charName]; ok {
		return true
	}
	return false
}

// The mob brain is this ticker
func (m *Mob) Tick() {
	// Am I actually in the room?
	if m.MobStunned > 0 {
		m.MobStunned -= 8
	} else {
		m.TicksAlive++
		if m.TicksAlive >= m.NumWander && m.CurrentTarget == "" {
			if !m.Flags["permanent"] {
				go Rooms[m.ParentId].WanderMob(m)
				return
			}
		} else {
			// Picking up treasure
			if m.Flags["takes_treasure"] {
				// Roll to see if I'm gonna pick it up
				if utils.Roll(100, 1, 0) <= config.MobTakeChance {
					// Loop inventory, and take the first thing they find
					for _, item := range Rooms[m.ParentId].Items.Contents {
						if m.Placement == item.Placement && !item.Flags["hidden"] {
							Rooms[m.ParentId].Items.Remove(item)
							m.Inventory.Add(item)
							Rooms[m.ParentId].MessageAll(m.Name + " picks up " + item.DisplayName() + text.Reset + "\n")
							return
						}
					}
				}
			}

			// Am I hostile?  Should I pick a target?
			if m.CurrentTarget == "" && m.Flags["hostile"] {
				potentials := Rooms[m.ParentId].Chars.MobList(m)
				if len(potentials) > 0 {
					rand.Seed(time.Now().Unix())
					m.CurrentTarget = potentials[rand.Intn(len(potentials))]
					m.AddThreatDamage(0, Rooms[m.ParentId].Chars.MobSearch(m.CurrentTarget, m))
					Rooms[m.ParentId].MessageAll(m.Name + " attacks " + m.CurrentTarget + text.Reset + "\n")
				}
			}

			if m.CurrentTarget != "" {
				if Rooms[m.ParentId].Chars.SearchAll(m.CurrentTarget) == nil {
					m.CurrentTarget = ""
				}
			}

			// Do I want to change targets? 33% chance if the current target isn't the highest on the threat table
			if len(m.ThreatTable) > 1 {
				rankedThreats := utils.RankMapStringInt(m.ThreatTable)
				if m.CurrentTarget != rankedThreats[0] {
					if utils.Roll(100, 1, 0) <= 15 {
						if utils.StringIn(rankedThreats[0], Rooms[m.ParentId].Chars.MobList(m)) {
							m.CurrentTarget = rankedThreats[0]
							Rooms[m.ParentId].MessageAll(m.Name + " turns to " + m.CurrentTarget + "\n" + text.Reset)
						}
					}
				}
			}

			if m.CurrentTarget == "" && m.Placement != 3 {
				oldPlacement := m.Placement
				if m.Placement > 3 {
					m.Placement--
				} else {
					m.Placement++
				}
				if !m.Flags["hidden"] {
					whichNumber := Rooms[m.ParentId].Mobs.GetNumber(m)
					if len(Rooms[m.ParentId].Mobs.Contents) > 1 && whichNumber > 1 {
						Rooms[m.ParentId].MessageMovement(oldPlacement, m.Placement, m.Name+" #"+strconv.Itoa(whichNumber))
					} else {
						Rooms[m.ParentId].MessageMovement(oldPlacement, m.Placement, m.Name)
					}
				}
				return
			}

			if m.CurrentTarget != "" && m.BreathWeapon != "" &&
				(math.Abs(float64(m.Placement-Rooms[m.ParentId].Chars.MobSearch(m.CurrentTarget, m).Placement)) == 1) {

				// Roll to see if we're going to breathe
				if utils.Roll(100, 1, 0) <= 30 {
					target := Rooms[m.ParentId].Chars.MobSearch(m.CurrentTarget, m)
					targets := []*Character{target}
					for _, character := range Rooms[m.ParentId].Chars.Contents {
						if character.Placement == target.Placement {
							targets = append(targets, character)
						}
					}

					Rooms[m.ParentId].MessageAll("The " + m.Name + " breathes " + m.BreathWeapon + " at " + target.Name + "\n")
					damageTotal := config.BreatheDamage(m.Level)
					reflectDamage := 0
					for _, t := range targets {
						if utils.StringIn(m.BreathWeapon, []string{"fire", "air", "earth", "water"}) {
							t.RunHook("attacked")
							t.ReceiveMagicDamage(damageTotal, m.BreathWeapon)
							t.Write([]byte("You take " + strconv.Itoa(damageTotal) + " damage from the " + m.BreathWeapon + " breath.\n"))
							if target.CheckFlag("reflection") {
								reflectDamage = int(float64(damageTotal) * (float64(target.GetStat("int")) * config.ReflectDamagePerInt))
								m.ReceiveDamage(reflectDamage)
								target.Write([]byte(text.Cyan + "You reflect " + strconv.Itoa(reflectDamage) + " damage back to " + m.Name + "!\n" + text.Reset))
								m.DeathCheck(target)
							}
							t.DeathCheck("was slain by a " + m.Name + ".")
						} else if m.BreathWeapon == "paralytic" {
							t.Write([]byte(text.Gray + m.Name + " breathes paralytic gas on to you.\n"))
							target.SetTimer("global", 24)
						} else if m.BreathWeapon == "pestilence" {
							t.Write([]byte(text.Gray + m.Name + " breathes infectious gas on to you.\n"))
							Effects["disease"](m, target, m.Level)
						}
					}
					return
				}
			}

			if m.CurrentTarget != "" && m.ChanceCast > 0 &&
				(math.Abs(float64(m.Placement-Rooms[m.ParentId].Chars.MobSearch(m.CurrentTarget, m).Placement)) >= 1) {
				// Try to cast a spell first
				target := Rooms[m.ParentId].Chars.MobSearch(m.CurrentTarget, m)
				spellSelected := false
				selectSpell := ""
				if utils.Roll(100, 1, 0) <= m.ChanceCast {
					for range m.Spells {
						rand.Seed(time.Now().Unix())
						selectSpell = m.Spells[rand.Intn(len(m.Spells))]
						if selectSpell != "" {
							if utils.StringIn(selectSpell, OffensiveSpells) {
								if m.Mana.Current > Spells[selectSpell].Cost {
									spellSelected = true
								}
							}
						}
					}

					if spellSelected {
						spellInstance, ok := Spells[selectSpell]
						if !ok {
							spellSelected = false
						}
						Rooms[m.ParentId].MessageAll(m.Name + " casts a " + spellInstance.Name + " spell on " + target.Name + "\n")
						target.RunHook("attacked")
						m.Mana.Subtract(spellInstance.Cost)
						result := Cast(m, target, spellInstance.Effect, spellInstance.Magnitude)
						if strings.Contains(result, "$SCRIPT") {
							m.MobScript(result)
						}
						target.DeathCheck("was slain by a " + m.Name + ".")
						return
					}
				}
			}

			// Calculate Vital/Crit/Double
			multiplier := float64(1)
			vitalStrike := false
			criticalStrike := false
			doubleDamage := false
			penalty := 1

			if m.CurrentTarget != "" && m.Flags["ranged_attack"] &&
				(math.Abs(float64(m.Placement-Rooms[m.ParentId].Chars.MobSearch(m.CurrentTarget, m).Placement)) >= 1) {
				target := Rooms[m.ParentId].Chars.MobSearch(m.CurrentTarget, m)
				missChance := 0
				lvlDiff := target.Tier - m.Level
				if lvlDiff >= 1 {
					missChance += lvlDiff * config.MissPerLevel
				}
				if utils.Roll(100, 1, 0) <= missChance {
					target.Write([]byte(text.Green + m.Name + " missed you!!" + "\n" + text.Reset))
					return
				}
				// If we made it here, default out and do a range hit.
				stamDamage := 0
				vitDamage := 0
				reflectDamage := 0
				actualDamage := m.InflictDamage()
				if target.GetStat("dex") < config.DexMinorPenalty {
					penalty = 2
				}
				if utils.Roll(100, 1, 0) <= config.MobVital*penalty {
					vitalStrike = true
				} else if utils.Roll(100, 1, 0) <= config.MobCritical*penalty {
					multiplier = 4
					criticalStrike = true
				} else if utils.Roll(100, 1, 0) <= config.MobDouble*penalty {
					multiplier = 2
					doubleDamage = true
				}
				if vitalStrike {
					vitDamage = target.ReceiveVitalDamage(int(math.Ceil(float64(actualDamage) * multiplier)))
					target.Write([]byte(text.Red + "Vital Strike!!!\n" + text.Reset))
				} else {
					stamDamage, vitDamage = target.ReceiveDamage(int(math.Ceil(float64(actualDamage) * multiplier)))
				}

				buildString := ""
				if stamDamage != 0 {
					buildString += strconv.Itoa(stamDamage) + " stamina"
				}
				if stamDamage != 0 && vitDamage != 0 {
					buildString += " and "
				}
				if vitDamage != 0 {
					buildString += strconv.Itoa(vitDamage) + " vitality"
				}
				if criticalStrike {
					target.Write([]byte(text.Red + "Critical Strike!!!\n" + text.Reset))
				}
				if doubleDamage {
					target.Write([]byte(text.Red + "Double Damage!!!\n" + text.Reset))
				}
				target.Write([]byte(text.Red + "Thwwip!! " + m.Name + " attacks you for " + buildString + " points of damage!" + "\n" + text.Reset))
				if target.CheckFlag("reflection") {
					reflectDamage = int(float64(actualDamage) * (float64(target.GetStat("int")) * config.ReflectDamagePerInt))
					m.ReceiveDamage(reflectDamage)
					target.Write([]byte(text.Cyan + "You reflect " + strconv.Itoa(reflectDamage) + " damage back to " + m.Name + "!\n" + text.Reset))
					m.DeathCheck(target)
				}
				target.RunHook("attacked")
				target.DeathCheck("was slain by a " + m.Name + ".")
				return
			}

			if (m.CurrentTarget != "" &&
				m.Placement != Rooms[m.ParentId].Chars.MobSearch(m.CurrentTarget, m).Placement) ||
				(m.CurrentTarget != "" &&
					(math.Abs(float64(m.Placement-Rooms[m.ParentId].Chars.MobSearch(m.CurrentTarget, m).Placement)) > 1)) {
				oldPlacement := m.Placement
				if m.Placement > Rooms[m.ParentId].Chars.MobSearch(m.CurrentTarget, m).Placement {
					m.Placement--
				} else {
					m.Placement++
				}
				if !m.Flags["hidden"] {
					whichNumber := Rooms[m.ParentId].Mobs.GetNumber(m)
					Rooms[m.ParentId].MessageMovement(oldPlacement, m.Placement, m.Name+" #"+strconv.Itoa(whichNumber))
				}
				// Next to attack
			} else if m.CurrentTarget != "" &&
				m.Placement == Rooms[m.ParentId].Chars.MobSearch(m.CurrentTarget, m).Placement {
				// Check to see if the mob misses:
				// Am I against a fighter and they succeed in a parry roll?
				target := Rooms[m.ParentId].Chars.MobSearch(m.CurrentTarget, m)
				missChance := 0
				lvlDiff := target.Tier - m.Level
				if lvlDiff >= 1 {
					missChance += lvlDiff * config.MissPerLevel
				}
				if utils.Roll(100, 1, 0) <= missChance {
					target.Write([]byte(text.Green + m.Name + " missed you!!" + "\n" + text.Reset))
					return
				}
				target.RunHook("attacked")
				m.CheckForExtraAttack(target)
				if target.Class == 0 && target.Equipment.Main != nil && config.RollParry(config.WeaponLevel(target.Skills[target.Equipment.Main.ItemType].Value, target.Class)) {
					if target.Tier >= 10 {
						// It's a riposte
						actualDamage, _ := m.ReceiveDamage(int(math.Ceil(float64(target.InflictDamage()))))
						target.Write([]byte(text.Green + "You parry and riposte the attack from " + m.Name + " for " + strconv.Itoa(actualDamage) + " damage!" + "\n" + text.Reset))
						if m.DeathCheck(target) {
							return
						}
						m.Stun(config.ParryStuns * 8)
					} else {
						target.Write([]byte(text.Green + "You parry the attack from " + m.Name + "\n" + text.Reset))
						m.Stun(config.ParryStuns * 8)
					}
				} else {
					stamDamage := 0
					vitDamage := 0
					actualDamage := m.InflictDamage()
					reflectDamage := 0
					if target.GetStat("dex") < config.DexMinorPenalty {
						penalty = 2
					}
					if utils.Roll(100, 1, 0) <= config.MobVital*penalty {
						vitalStrike = true
					} else if utils.Roll(100, 1, 0) <= config.MobCritical*penalty {
						multiplier = 4
						criticalStrike = true
					} else if utils.Roll(100, 1, 0) <= config.MobDouble*penalty {
						multiplier = 2
						doubleDamage = true
					}
					if vitalStrike {
						vitDamage = target.ReceiveVitalDamage(int(math.Ceil(float64(actualDamage) * multiplier)))
						target.Write([]byte(text.Red + "Vital Strike!!!\n" + text.Reset))
					} else {
						stamDamage, vitDamage = target.ReceiveDamage(int(math.Ceil(float64(actualDamage) * multiplier)))
					}
					buildString := ""
					if stamDamage != 0 {
						buildString += strconv.Itoa(stamDamage) + " stamina"
					}
					if stamDamage != 0 && vitDamage != 0 {
						buildString += " and "
					}
					if vitDamage != 0 {
						buildString += strconv.Itoa(vitDamage) + " vitality"
					}
					if stamDamage == 0 && vitDamage == 0 {
						target.Write([]byte(text.Red + m.Name + " attacks bounces off of you for no damage!" + "\n" + text.Reset))
					} else {
						if criticalStrike {
							target.Write([]byte(text.Red + "Critical Strike!!!\n" + text.Reset))
						}
						if doubleDamage {
							target.Write([]byte(text.Red + "Double Damage!!!\n" + text.Reset))
						}
						target.Write([]byte(text.Red + m.Name + " attacks you for " + buildString + " points of damage!" + "\n" + text.Reset))
					}
					if target.CheckFlag("reflection") {
						reflectDamage = int(float64(actualDamage) * (float64(target.GetStat("int")) * config.ReflectDamagePerInt))
						m.ReceiveDamage(reflectDamage)
						target.Write([]byte(text.Cyan + "You reflect " + strconv.Itoa(reflectDamage) + " damage back to " + m.Name + "!\n" + text.Reset))
						m.DeathCheck(target)
					}
					target.DeathCheck("was slain by a " + m.Name + ".")
				}
			}
		}
	}
}

func (m *Mob) DeathCheck(target *Character) bool {
	if m.Stam.Current <= 0 {
		Rooms[m.ParentId].MessageAll(text.Green + target.Name + " killed " + m.Name)
		stringExp := strconv.Itoa(m.Experience)
		for k := range m.ThreatTable {
			Rooms[m.ParentId].Chars.SearchAll(k).Write([]byte(text.Cyan + "You earn " + stringExp + "exp for the defeat of the " + m.Name + "\n" + text.Reset))
			Rooms[m.ParentId].Chars.SearchAll(k).Experience.Add(m.Experience)
		}
		Rooms[m.ParentId].MessageAll(m.Name + " dies.")
		target.Write([]byte(text.White + m.DropInventory()))
		go Rooms[m.ParentId].ClearMob(m)
		return true
	}
	return false
}

func (m *Mob) CheckForExtraAttack(target *Character) {

	if m.Flags["blinds"] {
		if utils.Roll(100, 1, 0) > 80 {
			target.Write([]byte(text.Red + m.Name + " blinds you!" + "\n" + text.Reset))
			Effects["blind"](m, target, 0)
			return
		}
	}

	if m.Flags["diseases"] {
		if utils.Roll(100, 1, 0) > 50 {
			target.Write([]byte(text.Red + m.Name + " tries to spread disease on to you!" + "\n" + text.Reset))
			Effects["disease"](m, target, m.Level)
			return
		}
	}
	if m.Flags["poisons"] {
		if utils.Roll(100, 1, 0) > 50 {
			target.Write([]byte(text.Red + m.Name + " injects you with venom!" + "\n" + text.Reset))
			Effects["poison"](m, target, m.Level)
			return
		}
	}
	if m.Flags["spits_acid"] {
		if target.CheckFlag("resilient-aura") {
			target.Write([]byte(text.Red + m.Name + " spits acid on you, but your aura protects your gear!" + "\n" + text.Reset))
			return
		}
		if utils.Roll(100, 1, 0) > 50 {
			target.Write([]byte(text.Red + m.Name + " spits acid on you, damaging your armor !" + "\n" + text.Reset))
			msg := target.Equipment.DamageRandomArmor()
			if msg != "" {
				target.Write([]byte(text.Info + msg + "\n" + text.Reset))
			}
			return
		}
	}
	return
}

func (m *Mob) Follow(params []string) {
	// Am I still the most mad at the guy who left?  I could have gotten bored with that...
	if params[0] == m.CurrentTarget && m.MobStunned == 0 {
		//log.Println("I'm gonna try to follow")
		// I am, lets process that -> First we need to step up in the world to find that character
		targetChar := ActiveCharacters.Find(params[0])
		if targetChar != nil {
			curChance := config.MobFollow - ((targetChar.Tier - m.Level) * config.MobFollowPerLevel)
			if curChance > 85 {
				curChance = 85
			}
			if utils.Roll(100, 1, 0) <= curChance {
				log.Println("I'm gonna follow")
				// Halt processing
				neededLocks := make([]int, 2)
				neededLocks[0] = m.ParentId
				neededLocks[1] = targetChar.ParentId
				ready := false
				previousRoom := m.ParentId
				Rooms[m.ParentId].StagedClearing = false
				// Lets not compete with other mobs for the same locks by using names
				//log.Println("Mob is trying to gain lock priority")
				tempName := utils.RandString(10)
				for !ready {
					ready = true
					for _, l := range neededLocks {
						if Rooms[l].LockPriority == "" {
							Rooms[l].LockPriority = tempName
						} else if Rooms[l].LockPriority != tempName {
							ready = false
						}
					}
					if !ready {
						for _, l := range neededLocks {
							//log.Println(s.actor.Name + " releasing incomplete priority" + strconv.Itoa(l))
							if Rooms[l].LockPriority == tempName {
								Rooms[l].LockPriority = ""
							}
						}
						//log.Println("Mob is sleeping before trying to acquire again")
						rand.Seed(time.Now().UnixNano())
						r := rand.Int()
						t, _ := time.ParseDuration(string(r) + "ms")
						time.Sleep(t)
					}
				}
				//log.Println("Let everyone know there is a follow")
				Rooms[m.ParentId].MessageAll(m.Name + " follows " + targetChar.Name)
				//log.Println("Processing Previous Room Lock")
				Rooms[m.ParentId].Lock()
				//log.Println("Processing New Room Lock")
				Rooms[targetChar.ParentId].Lock()
				targetChar.Write([]byte(text.Bad + m.Name + " follows you." + text.Reset + "\n"))
				//log.Println("Remove mob")
				Rooms[m.ParentId].Mobs.Remove(m)
				//log.Println("Add the mob")
				Rooms[targetChar.ParentId].Mobs.AddWithMessage(m, m.Name+" follows "+targetChar.Name+" into the area.", false)
				//log.Println("Check for Vital")
				if utils.Roll(100, 1, 0) <= config.MobFollowVital {
					vitDamage := targetChar.ReceiveVitalDamage(int(math.Ceil(float64(m.InflictDamage()))))
					if vitDamage == 0 {
						targetChar.Write([]byte(text.Red + m.Name + " attacks bounces off of you for no damage!" + "\n" + text.Reset))
					} else {
						targetChar.Write([]byte(text.Red + "Vital Strike!!!!\n" + text.Reset))
						targetChar.Write([]byte(text.Red + m.Name + " attacks you for " + strconv.Itoa(vitDamage) + " points of vital damage!" + "\n" + text.Reset))
					}
					targetChar.DeathCheck("was slain by a " + m.Name + ".")
				}
				//log.Println("Previous room Unlock")
				Rooms[previousRoom].Unlock()
				//log.Println("New Room Unlock")
				Rooms[targetChar.ParentId].Unlock()
				//log.Println("Release lock priorities")
				for _, l := range neededLocks {
					Rooms[l].LockPriority = ""
				}
				// Clean the previous room
				go Rooms[previousRoom].LastPerson()
				go func() {
					time.Sleep(1 * time.Second)
					m.StartTicking()
				}()

				m.CurrentTarget = targetChar.Name
			}
		}
	}
}

func (m *Mob) Flee(params []string) {
	// Roll a dice and see if I'm going to flee...
	if m.CheckFlag("sweet_comfort") {
		return
	}
	if utils.Roll(100, 1, 0) <= 50 {
		go Rooms[m.ParentId].FleeMob(m)
	}
	return
}

func (m *Mob) ProcessCommand(cmdStr string, params []string) {
	// StateCommands is a list of stack functions that cause the mob to do something that affects state and causes actions
	log.Println("Processing command " + cmdStr)
	var StateCommands = map[string]func(params []string){
		"attack":   nil,
		"cast":     nil,
		"move":     nil,
		"follow":   m.Follow,
		"pickup":   nil,
		"teleport": nil,
		"flee":     m.Flee,
	}
	StateCommands[cmdStr](params)
}

func (m *Mob) MobScript(inputStr string) {
	input := strings.Split(inputStr, " ")
	switch input[0] {
	case "$TELEPORT":
		m.Teleport(strings.Join(input[1:], " "))
	}

}

func (m *Mob) Stun(amt int) {
	// TODO: Diminishing returns and max stun length
	if amt > m.MobStunned {
		m.MobStunned += amt
	}
}

// Special handler for handling a mobs cast of a teleport spell
func (m *Mob) Teleport(target string) {
	rand.Seed(time.Now().Unix())
	newRoom := Rooms[TeleportTable[rand.Intn(len(TeleportTable))]]
	targetName := strings.Split(target, " ")
	workingRoom := Rooms[m.ParentId]
	targetChar := workingRoom.Chars.SearchAll(targetName[0])
	if targetChar != nil {
		if targetChar.Resist {
			// For every 5 points of int over the target there's an extra 10% chance to teleport
			diff := (m.Level - targetChar.Tier) * 5
			chance := 10 + diff
			if utils.Roll(100, 1, 0) > chance {
				targetChar.Write([]byte(m.Name + " failed to teleport you.\n"))
				return
			}
		}
		targetChar.Write([]byte(m.Name + " teleported you.\n"))
		newRoom.Lock()
		workingRoom.Chars.Remove(targetChar)
		newRoom.Chars.Add(targetChar)
		targetChar.ParentId = newRoom.RoomId
		targetChar.Write([]byte(newRoom.Look(targetChar)))
		newRoom.Unlock()
	}
}

// On copy to a room calculate the inventory
func (m *Mob) CalculateInventory() {
	if len(m.ItemList) > 0 {
		for k, v := range m.ItemList {
			if utils.Roll(100, 1, 0) <= v {
				// Successful roll!  Add this item to the inventory!
				newItem := Item{}
				copier.CopyWithOption(&newItem, Items[k], copier.Option{DeepCopy: true})
				m.Inventory.Add(&newItem)
			}
		}
	}
}

func (m *Mob) ReturnState() string {
	stamStatus := "healthy"
	if m.Stam.Current < (m.Stam.Max - int(.90*float32(m.Stam.Max))) {
		stamStatus = "near death"
	} else if m.Stam.Current < (m.Stam.Max - int(.75*float32(m.Stam.Max))) {
		stamStatus = "badly injured"
	} else if m.Stam.Current < (m.Stam.Max - int(.5*float32(m.Stam.Max))) {
		stamStatus = "injured"
	} else if m.Stam.Current < (m.Stam.Max - int(.25*float32(m.Stam.Max))) {
		stamStatus = "slightly injured"
	}
	return " looks " + stamStatus
}

func (m *Mob) DropInventory() string {
	var drops []string
	var tempStore []*Item
	for _, item := range m.Inventory.Contents {
		tempStore = append(tempStore, item)
	}
	if len(tempStore) > 0 {
		for _, item := range tempStore {
			if item != nil {
				if err := m.Inventory.Remove(item); err == nil {
					if len(Rooms[m.ParentId].Items.Contents) < 15 {
						item.Placement = m.Placement
						Rooms[m.ParentId].Items.Add(item)
						drops = append(drops, item.Name)
					} else {
						Rooms[m.ParentId].MessageAll(item.Name + " fall on top of other items and rolls away.\n" + text.Reset)
					}
				}
			}
		}
	}
	if m.Gold > 0 {
		newGold := Item{}
		copier.CopyWithOption(&newGold, Items[3456], copier.Option{DeepCopy: true})
		newGold.Name = strconv.Itoa(m.Gold) + " gold marks"
		newGold.Value = m.Gold
		newGold.Placement = m.Placement
		Rooms[m.ParentId].Items.Add(&newGold)
		drops = append(drops, newGold.Name)
	}
	if len(drops) == 0 {
		return "The " + m.Name + " was carrying:\n Nothing"
	} else {
		return "The " + m.Name + " was carrying:\n " + strings.Join(drops, ", ")
	}
}

func (m *Mob) AddThreatDamage(damage int, attacker *Character) {
	if !attacker.Permission.HasAnyFlags(permissions.Builder, permissions.Dungeonmaster, permissions.Gamemaster) {
		m.ThreatTable[attacker.Name] += damage
		if m.CurrentTarget == "" {
			m.CurrentTarget = attacker.Name
		}
	}
}

func (m *Mob) ApplyEffect(effectName string, length string, interval int, magnitude int, effect func(triggers int), effectOff func()) {
	if effectInstance, ok := m.Effects[effectName]; ok {
		durExtend, _ := strconv.ParseFloat(length, 64)
		effectInstance.ExtendDuration(durExtend)
		return
		//m.Effects[effectName].effectOff()
	}
	m.Effects[effectName] = NewEffect(length, interval, magnitude, effect, effectOff)
	m.Effects[effectName].RunEffect()
}

func (m *Mob) RemoveEffect(effectName string) {
	delete(m.Effects, effectName)
}

func (m *Mob) ApplyHook(hook string, hookName string, executions int, length string, interval int, effect func(), effectOff func()) {
	m.Hooks[hook][hookName] = NewHook(executions, length, interval, effect, effectOff)
}

func (m *Mob) RemoveHook(hook string, hookName string) {
	m.Hooks[hook][hookName].effectOff()
	valPresent := false
	for k, _ := range m.Hooks {
		valPresent = false
		for hName, _ := range m.Hooks[k] {
			if hName == hookName {
				valPresent = true
			}
		}
		if valPresent {
			delete(m.Hooks[k], hookName)
		}
	}
}

func (m *Mob) RunHook(hook string) {
	for name, hookInstance := range m.Hooks[hook] {
		// Process Removing the hook
		if hookInstance.TimeRemaining() == 0 {
			m.RemoveHook(hook, name)
			continue
		}
		if hookInstance.interval > 0 {
			//log.Println(hookInstance.LastTriggerInterval())
			if hookInstance.LastTriggerInterval() <= 0 {
				hookInstance.RunHook()
			}
		} else if hookInstance.interval == -1 {
			hookInstance.RunHook()
		}
	}
	return
}

func (m *Mob) GetInt() int {
	return m.Int.Current
}

func (m *Mob) ToggleFlag(flagName string) bool {
	if val, exists := m.Flags[flagName]; exists {
		m.Flags[flagName] = !val
		return true
	} else {
		return false
	}
}

func (m *Mob) ToggleFlagAndMsg(flagName string, msg string) {
	if val, exists := m.Flags[flagName]; exists {
		m.Flags[flagName] = !val
	} else {
		m.Flags[flagName] = true
	}
	log.Println(m.Name, " informed: ", msg)
}

func (m *Mob) CheckFlag(flagName string) bool {
	if _, ok := m.Flags[flagName]; ok {
		return m.Flags[flagName]
	}
	return false
}

func (m *Mob) ReceiveDamage(damage int) (int, int) {
	resist := int(math.Ceil((float64(m.Armor/config.MobArmorReductionPoints) * config.MobArmorReduction) * float64(damage)))
	finalDamage := damage - resist
	if m.CheckFlag("inertial-barrier") {
		finalDamage -= int(math.Ceil(float64(damage) * config.InertialDamageIgnore))
	}
	m.Stam.Subtract(finalDamage)
	if finalDamage > m.WimpyValue && m.CheckFlag("flees") {
		m.MobCommands <- "flee"
	}
	log.Println(m.Name+" Receives Damage: ", damage, "Resist: ", resist, "Final Damage: ", finalDamage)
	return finalDamage, 0
}

func (m *Mob) ReceiveDamageNoArmor(damage int) (int, int) {
	finalDamage := int(math.Ceil(float64(damage)))
	m.Stam.Subtract(finalDamage)
	if finalDamage > m.WimpyValue && m.CheckFlag("flees") {
		m.MobCommands <- "flee"
	}
	return finalDamage, 0
}

func (m *Mob) ReceiveMagicDamage(damage int, element string) (int, int, int) {
	var resisting float64

	switch element {
	case "fire":
		resisting = float64(m.FireResistance) / 100
		if m.CheckFlag("resist_fire") {
			resisting += .25
		}
	case "air":
		resisting = float64(m.AirResistance) / 100
		if m.CheckFlag("resist_air") {
			resisting += .25
		}
	case "earth":
		resisting = float64(m.EarthResistance) / 100
		if m.CheckFlag("resist_earth") {
			resisting += .25
		}
	case "water":
		resisting = float64(m.WaterResistance) / 100
		if m.CheckFlag("resist_water") {
			resisting += .25
		}
	}
	if resisting > 0 {
		resisting = (float64(m.Int.Current) / 30) * resisting
	}

	if m.CheckFlag("resist_magic") {
		resisting += .10
	}
	resisted := int(math.Ceil(float64(damage) * resisting))
	stamDam, vitDam := m.ReceiveDamageNoArmor(damage - resisted)
	return stamDam, vitDam, resisted
}

func (m *Mob) ReceiveVitalDamage(damage int) int {
	damageMod, _ := m.ReceiveDamage(damage)
	return damageMod
}

func (m *Mob) Heal(damage int) (int, int) {
	m.Stam.Add(damage)
	return damage, 0
}

func (m *Mob) HealStam(damage int) {
	m.Stam.Add(damage)
}

func (m *Mob) HealVital(damage int) {
	m.Heal(damage)
}

func (m *Mob) RestoreMana(damage int) {
	m.Mana.Add(damage)
}

func (m *Mob) InflictDamage() int {
	damage := 0
	if m.NumDice > 0 && m.SidesDice > 0 {
		damage = utils.Roll(m.SidesDice, m.NumDice, m.PlusDice)
	}
	return damage
}

func (m *Mob) CastSpell(spell string) bool {
	return true
}

func (m *Mob) Look() string {
	buildText := "You see a " + m.Name + ", " + config.TextTiers[m.Level] + " level. \n"
	buildText += m.Description
	if m.Flags["hostile"] {
		buildText += "\n It looks hostile!"
	}
	return buildText
}

// Function to return only the modifiable properties
func ReturnMobInstanceProps(mob *Mob) map[string]interface{} {
	serialList := map[string]interface{}{
		"mobId":     mob.MobId,
		"health":    mob.Stam.Current,
		"mana":      mob.Mana.Current,
		"placement": mob.Placement,
		"inventory": mob.Inventory.Jsonify(),
	}
	return serialList
}

func (m *Mob) Save() {
	mobData := make(map[string]interface{})
	mobData["mob_id"] = m.MobId
	mobData["name"] = m.Name
	mobData["description"] = m.Description
	mobData["experience"] = m.Experience
	mobData["level"] = m.Level
	mobData["gold"] = m.Gold
	mobData["constitution"] = m.Con.Current
	mobData["strength"] = m.Str.Current
	mobData["intelligence"] = m.Int.Current
	mobData["dexterity"] = m.Dex.Current
	mobData["piety"] = m.Pie.Current
	mobData["mpmax"] = m.Mana.Max
	mobData["mpcur"] = m.Mana.Current
	mobData["hpcur"] = m.Stam.Current
	mobData["hpmax"] = m.Stam.Max
	mobData["sdice"] = m.SidesDice
	mobData["ndice"] = m.NumDice
	mobData["pdice"] = m.PlusDice
	mobData["spells"] = strings.Join(m.Spells, ",")
	mobData["casting_probability"] = m.ChanceCast
	mobData["armor"] = m.Armor
	mobData["numwander"] = m.NumWander
	mobData["wimpyvalue"] = m.WimpyValue
	mobData["air_resistance"] = m.AirResistance
	mobData["fire_resistance"] = m.FireResistance
	mobData["earth_resistance"] = m.EarthResistance
	mobData["water_resistance"] = m.WaterResistance
	mobData["hide_encounter"] = utils.Btoi(m.Flags["hide_encounter"])
	mobData["invisible"] = utils.Btoi(m.Flags["invisible"])
	mobData["permanent"] = utils.Btoi(m.Flags["permanent"])
	mobData["hostile"] = utils.Btoi(m.Flags["hostile"])
	mobData["undead"] = utils.Btoi(m.Flags["undead"])
	mobData["breathes"] = m.BreathWeapon
	mobData["fast_moving"] = utils.Btoi(m.Flags["fast_moving"])
	mobData["night_only"] = utils.Btoi(m.Flags["night_only"])
	mobData["day_only"] = utils.Btoi(m.Flags["day_only"])
	mobData["guard_treasure"] = utils.Btoi(m.Flags["guard_treasure"])
	mobData["take_treasure"] = utils.Btoi(m.Flags["take_treasure"])
	mobData["steals"] = utils.Btoi(m.Flags["steals"])
	mobData["block_exit"] = utils.Btoi(m.Flags["block_exit"])
	mobData["follows"] = utils.Btoi(m.Flags["follows"])
	mobData["no_steal"] = utils.Btoi(m.Flags["no_steal"])
	mobData["detect_invisible"] = utils.Btoi(m.Flags["detect_invisible"])
	mobData["no_stun"] = utils.Btoi(m.Flags["no_stun"])
	mobData["diseases"] = utils.Btoi(m.Flags["diseases"])
	mobData["poisons"] = utils.Btoi(m.Flags["poisons"])
	mobData["spits_acid"] = utils.Btoi(m.Flags["spits_acid"])
	mobData["ranged_attack"] = utils.Btoi(m.Flags["ranged_attack"])
	mobData["flees"] = utils.Btoi(m.Flags["flees"])
	mobData["blinds"] = utils.Btoi(m.Flags["blinds"])
	data.UpdateMob(mobData)
}

func (m *Mob) Eval() string {
	return "You study the " + m.Name + " in your minds eye....\n\n" +
		"It is level " + strconv.Itoa(m.Level) + ". \n" +
		"It currently has " + strconv.Itoa(m.Stam.Current) + " hits points remaining. \n" +
		"It is worth " + strconv.Itoa(m.Experience) + " experience points. \n"
}
