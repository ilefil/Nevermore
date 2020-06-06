package cmd

import (
	"github.com/ArcCS/Nevermore/permissions"
	"github.com/ArcCS/Nevermore/stats"
	"strings"
)

func init() {
	addHandler(appeal{},
		"Usage:  appeal HELP ME OH GODS OR CREATORS\n \n Appeal a message to higher powers.  Note: Preppend OOC to clarify it's a non RP issue", 				permissions.Player,
		"APPEAL")

}

type appeal cmd

func (appeal) process(s *state) {
	if len(s.words) == 0 {
		s.msg.Actor.SendInfo("What did you want to say?")
		return
	}

	stats.ActiveCharacters.MessageGM("### " + s.actor.Name + " appeals: " + strings.Join(s.input, " "))

	s.ok = true
	return
}
