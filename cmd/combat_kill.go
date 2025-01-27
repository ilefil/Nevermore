package cmd

import (
	"github.com/ArcCS/Nevermore/config"
	"github.com/ArcCS/Nevermore/objects"
	"github.com/ArcCS/Nevermore/permissions"
	"github.com/ArcCS/Nevermore/text"
	"github.com/ArcCS/Nevermore/utils"
	"log"
	"math"
	"strconv"
)

func init() {
	addHandler(kill{},
		"Usage:  kill target # \n\n Try to attack something! Can also use attack, or shorthand k",
		permissions.Player,
		"kill", "k")
}

type kill cmd

func (kill) process(s *state) {
	if len(s.input) < 1 && s.actor.Victim == nil {
		s.msg.Actor.SendBad("Attack what exactly?")
		return
	}

	if s.actor.CheckFlag("blind") {
		s.msg.Actor.SendBad("You can't see anything!")
		return
	}

	if s.actor.Stam.Current <= 0 {
		s.msg.Actor.SendBad("You are far too tired to do that.")
		return
	}

	// Check some timers
	ready, msg := s.actor.TimerReady("combat")
	if !ready {
		s.msg.Actor.SendBad(msg)
		return
	}

	name := ""
	nameNum := 1
	if len(s.words) < 1 && s.actor.Victim != nil {
		switch s.actor.Victim.(type) {
		case *objects.Character:
			name = s.actor.Victim.(*objects.Character).Name
		case *objects.Mob:
			name = s.actor.Victim.(*objects.Mob).Name
			nameNum = s.where.Mobs.GetNumber(s.actor.Victim.(*objects.Mob))
		}
	} else {
		name = s.input[0]
	}

	if len(s.words) > 1 {
		// Try to snag a number off the list
		if val, err := strconv.Atoi(s.words[1]); err == nil {
			nameNum = val
		}
	}

	var whatMob *objects.Mob
	whatMob = s.where.Mobs.Search(name, nameNum, s.actor)
	if whatMob != nil {
		s.actor.Victim = whatMob

		// This is an override for a GM to delete a mob
		if s.actor.Permission.HasAnyFlags(permissions.Builder, permissions.Dungeonmaster, permissions.Gamemaster) {
			s.msg.Actor.SendInfo("You smashed ", whatMob.Name, " out of existence.")
			objects.Rooms[whatMob.ParentId].Mobs.Remove(whatMob)
			whatMob = nil
			return
		}

		s.actor.RunHook("combat")

		// Shortcut a missing weapon:
		if s.actor.Equipment.Main == (*objects.Item)(nil) && s.actor.Class != 8 {
			s.msg.Actor.SendBad("You have no weapon to attack with.")
			return
		}

		if _, err := whatMob.ThreatTable[s.actor.Name]; !err {
			s.msg.Actor.Send(text.White + "You engaged " + whatMob.Name + " #" + strconv.Itoa(s.where.Mobs.GetNumber(whatMob)) + " in combat.")
			s.msg.Observers.Send(text.White + s.actor.Name + " attacks " + whatMob.Name)
			whatMob.AddThreatDamage(0, s.actor)
		}

		if s.actor.Class != 8 {
			// Shortcut target not being in the right location, check if it's a missile weapon, or that they are placed right.
			if (s.actor.Equipment.Main.ItemType != 4 && s.actor.Equipment.Main.ItemType != 3) && (s.actor.Placement != whatMob.Placement) {
				s.msg.Actor.SendBad("You are too far away to attack.")
				return
			} else if s.actor.Equipment.Main.ItemType == 4 && (s.actor.Placement == whatMob.Placement) {
				s.msg.Actor.SendBad("You are too close to attack.")
				return
			} else if s.actor.Equipment.Main.ItemType == 3 && (s.actor.Placement == whatMob.Placement) {
				s.msg.Actor.SendBad("You are too close to attack.")
				return
			} else if s.actor.Equipment.Main.ItemType == 3 && (int(math.Abs(float64(s.actor.Placement-whatMob.Placement))) > 1) {
				s.msg.Actor.SendBad("You are too far away to attack.")
				return
			}
		} else {
			if s.actor.Placement != whatMob.Placement {
				s.msg.Actor.SendBad("You are too far away to attack.")
				return
			}
		}

		// use a list of attacks,  so we can expand this later if other classes get multi style attacks
		attacks := []float64{
			1.0,
		}

		skillLevel := config.WeaponLevel(s.actor.Skills[5].Value, s.actor.Class)
		if s.actor.Class != 8 {
			skillLevel = config.WeaponLevel(s.actor.Skills[s.actor.Equipment.Main.ItemType].Value, s.actor.Class)
		}
		// Kill is really the fighters realm for specialty..
		if s.actor.Permission.HasAnyFlags(permissions.Fighter) {
			// mob lethal?
			if config.RollLethal(skillLevel) {
				// Sure did.  Kill this fool and bail.
				s.msg.Actor.SendInfo("You landed a lethal blow on the " + whatMob.Name)
				s.msg.Observers.SendInfo(s.actor.Name + " landed a lethal blow on " + whatMob.Name)
				s.actor.Equipment.DamageWeapon("main", 1)
				whatMob.Stam.Current = 0
				DeathCheck(s, whatMob)
				s.actor.SetTimer("combat", 8)
				return
			}

			if skillLevel >= 4 {
				attacks = append(attacks, .15)
				if skillLevel >= 5 {
					attacks[1] = .3
					if skillLevel >= 6 {
						attacks = append(attacks, .15)
						if skillLevel >= 7 {
							attacks[2] = .30
							if skillLevel >= 8 {
								attacks = append(attacks, .15)
								if skillLevel >= 9 {
									attacks[3] = .3
									if skillLevel >= 10 {
										attacks = append(attacks, .30)
									}
								}
							}
						}
					}
				}
			}
		}

		// start executing the attacks
		weaponDamage := 1
		weapMsg := ""
		alwaysCrit := false
		if s.actor.Class != 8 {
			alwaysCrit = s.actor.Equipment.Main.Flags["always_crit"]
		}
		for _, mult := range attacks {
			// Check for a miss
			if utils.Roll(100, 1, 0) <= DetermineMissChance(s, whatMob.Level-s.actor.Tier) {
				s.msg.Actor.SendBad("You missed!!")
				s.actor.SetTimer("combat", config.CombatCooldown)
				return
			} else {
				if config.RollCritical(skillLevel) || alwaysCrit {
					mult *= float64(config.CombatModifiers["critical"])
					s.msg.Actor.SendGood("Critical Strike!")
					weaponDamage = 10
				} else if config.RollDouble(skillLevel) {
					mult *= float64(config.CombatModifiers["double"])
					s.msg.Actor.SendGood("Double Damage!")
				}
				// Str Penalty Check
				if s.actor.Class != 8 {
					if s.actor.Equipment.Main.ItemType == 4 && s.actor.GetStat("str") < config.StrMajorPenalty {
						selfDamage, vitDamage := s.actor.ReceiveDamage(int(math.Ceil(float64(s.actor.InflictDamage()) * config.StrRangePenaltyDamage)))
						s.msg.Actor.SendBad("You aren't strong enough to handle the after-effects of the weapon and hit yourself for " + strconv.Itoa(selfDamage) + "stamina and " + strconv.Itoa(vitDamage) + " damage!")
						s.actor.DeathCheck(" killed themselves from the kickback of their weapon.")
					} else if s.actor.Equipment.Main.ItemType == 4 && s.actor.GetStat("str") < config.StrMinorPenalty {
						if utils.Roll(100, 1, 0) <= config.StrMinorPenaltyChance {
							selfDamage, vitDamage := s.actor.ReceiveDamage(int(math.Ceil(float64(s.actor.InflictDamage()) * config.StrRangePenaltyDamage)))
							s.msg.Actor.SendBad("You aren't strong enough to handle the after-effecs of the weapon and hit yourself for " + strconv.Itoa(selfDamage) + "stamina and " + strconv.Itoa(vitDamage) + " damage!")
							s.actor.DeathCheck(" killed themselves from the kickback of their weapon.")
						} else {
							s.msg.Actor.SendGood("You narrowely avoid hitting yourself with your ranged weapon.")
						}
					}
				}

				actualDamage, _ := whatMob.ReceiveDamage(int(math.Ceil(float64(s.actor.InflictDamage()) * mult)))
				whatMob.AddThreatDamage(actualDamage, s.actor)
				log.Println(strconv.Itoa(whatMob.Stam.Max))
				s.actor.AdvanceSkillExp(int((float64(actualDamage) / float64(whatMob.Stam.Max) * float64(whatMob.Experience)) * config.Classes[config.AvailableClasses[s.actor.Class]].WeaponAdvancement))
				s.msg.Actor.SendInfo("You hit the " + whatMob.Name + " for " + strconv.Itoa(actualDamage) + " damage!" + text.Reset)
				if whatMob.CheckFlag("reflection") {
					reflectDamage := int(float64(actualDamage) * config.ReflectDamageFromMob)
					s.actor.ReceiveDamage(reflectDamage)
					s.msg.Actor.Send("The " + whatMob.Name + " reflects " + strconv.Itoa(reflectDamage) + " damage back at you!")
					s.actor.DeathCheck(" was killed by reflection!")
				}
			}
		}
		DeathCheck(s, whatMob)
		if s.actor.Class != 8 {
			weapMsg = s.actor.Equipment.DamageWeapon("main", weaponDamage)
			if weapMsg != "" {
				s.msg.Actor.SendInfo(weapMsg)
			}
		}

		s.actor.SetTimer("combat", config.CombatCooldown)
		return

	}

	s.msg.Actor.SendInfo("Attack what?")
	s.ok = true
}

// DeathCheck Universal death check for mobs on whatever the current state is
func DeathCheck(s *state, m *objects.Mob) {
	if m.Stam.Current <= 0 {
		s.msg.Actor.SendGood("You killed " + m.Name)
		s.msg.Observers.SendGood(s.actor.Name + " killed " + m.Name)
		for k, threat := range m.ThreatTable {
			buildActorString := ""
			charClean := s.where.Chars.SearchAll(k)
			if charClean != nil {
				if threat > 0 {
					buildActorString += text.Cyan + "You earn " + strconv.Itoa(m.Experience) + " experience for the defeat of the " + m.Name + "\n"
					charClean.Experience.Add(m.Experience)
				}
				if charClean == s.actor {
					buildActorString += text.Green + m.DropInventory() + "\n"
					s.msg.Actor.Send(buildActorString)
				} else {
					charClean.Write([]byte(buildActorString + "\n" + text.Reset))
				}
				if charClean.Victim == m {
					charClean.Victim = nil
				}
			}
		}

		s.where.Mobs.Remove(m)
	}
}

// Determine Miss Chance based on weapon Skills
func DetermineMissChance(s *state, lvlDiff int) int {
	missChance := 0
	if s.actor.Class == 8 {
		missChance = config.WeaponMissChance(s.actor.Skills[5].Value, s.actor.Class)
	} else {
		missChance = config.WeaponMissChance(s.actor.Skills[s.actor.Equipment.Main.ItemType].Value, s.actor.Class)
	}
	if lvlDiff >= 1 {
		missChance += lvlDiff * config.MissPerLevel
	}
	if missChance >= 100 {
		missChance = 95
	}
	return missChance
}
