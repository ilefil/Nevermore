package cmd

import (
	"github.com/ArcCS/Nevermore/objects"
	"github.com/ArcCS/Nevermore/permissions"
	"github.com/ArcCS/Nevermore/text"
	"strings"
)

func init() {
	addHandler(tell{},
		"Usage:  send character_name \n \n Send a telepathic message to another player",
		permissions.Player,
		"TELL", "SEND")
}

type tell cmd

func (tell) process(s *state) {
	if len(s.words) < 2 {
		s.msg.Actor.SendBad("Send what to who?")
		return
	}
	whoStr := s.words[0]
	message := strings.Join(s.input[1:], " ")
	who := objects.ActiveCharacters.Find(whoStr)
	if who != nil {
		objects.ActiveCharacters.Lock()
		who.LastMessenger = s.actor.Name
		who.Write([]byte(text.White + s.actor.Name + " flashes#, \"" + message + "\"" + text.Reset + "\n"))
		objects.ActiveCharacters.Unlock()
		if !who.Flags["invisible"] {
			s.msg.Actor.SendGood("You sent#, \"" + message + "\", to " + who.Name)
		} else {
			s.msg.Actor.SendBad("Send what to who?")
		}
	} else {
		s.msg.Actor.SendBad("Send what to who?")
	}

	s.ok = true
	return
}
