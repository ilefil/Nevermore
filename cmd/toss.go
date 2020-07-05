package cmd

import (
	"github.com/ArcCS/Nevermore/permissions"
	"strconv"
)

// Syntax: JUNK item
func init() {
	addHandler(toss{},
           "Usage:  toss itemName # \n \n Toss an item away, this is a permanent deletion.",
           permissions.Player,
           "toss")
}

type toss cmd

func (j toss) process(s *state) {

	var numId int
	var err error

	if len(s.words) == 0 {
		s.msg.Actor.SendInfo("What do you want to toss?")
		return
	}

	name := s.words[0]
	if numId, err = strconv.Atoi(s.words[1]); err != nil {
		numId = 1
	}

	// Search for item we want to junk in our inventory
	where := s.actor.Inventory
	what := where.Search(name, numId)

	// Still not found?
	if what == nil {
		s.msg.Actor.SendBad("You see no '", name, "' to junk.")
		return
	}

	// Get item's proper name
	name = what.Name

	// Check junking is not vetoed by the item
	if veto := what.Flags["tossable"]; veto == false {
		s.msg.Actor.SendBad(name, "cannot be tossed away.")
		return
	}

	// Unlock, remote, relock, free item
	where.Unlock()
	where.Remove(what)
	where.Lock()
	what.Free()

	who := s.actor.Name

	s.msg.Actor.SendGood("You toss away ", name, ".")
	s.msg.Observers.SendInfo("You see ", who, " toss away ", name, ".")
	s.ok = true
}
