package network

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/httpapi"
)

const (
	maxMessage = 256
	// maxCommand is the longest console command agents take.
	maxCommand = 1000
	// Everyone is the target of the players of a whole server.
	Everyone = "@a"
)

// Message is what game servers show their players: in the chat, as a title with an optional
// subtitle, or above the hotbar.
type Message struct {
	// Kind is chat (also when empty), title or actionbar.
	Kind     string `json:"kind"`
	Text     string `json:"message"`
	Subtitle string `json:"subtitle"`
}

// textComponent is a text of Minecraft's commands, which it reads as JSON, and as SNBT since
// Minecraft 1.21.5, of which JSON is part.
type textComponent struct {
	Text      string   `json:"text,omitempty"`
	Translate string   `json:"translate,omitempty"`
	With      []string `json:"with,omitempty"`
	Color     string   `json:"color,omitempty"`
	Italic    bool     `json:"italic,omitempty"`
}

// Commands returns the console commands that show the message to a player or to Everyone.
// The chat of everyone is say, which shows [Server] before the text; a player gets it as a
// whisper of the server, like with msg. Texts become JSON with json.Marshal, never by hand.
func (m Message) Commands(target string) ([]string, error) {
	text, subtitle := strings.TrimSpace(m.Text), strings.TrimSpace(m.Subtitle)
	switch {
	case target != Everyone && !noryxv1.ValidPlayerName(target):
		return nil, httpapi.Errorf(http.StatusBadRequest, "Enter the name of a player: up to 16 letters, digits and underscores.")
	case text == "" || !validText(text) || !validText(subtitle):
		return nil, httpapi.Errorf(http.StatusBadRequest, "Enter a message of up to %d characters on one line.", maxMessage)
	case subtitle != "" && m.Kind != "title":
		return nil, httpapi.Errorf(http.StatusBadRequest, "Only titles have a subtitle.")
	}
	var commands []string
	switch m.Kind {
	case "", "chat":
		if target == Everyone {
			return []string{"say " + text}, nil
		}
		commands = append(commands, "minecraft:tellraw "+target+" "+component(textComponent{
			Translate: "commands.message.display.incoming", With: []string{"Server", text}, Color: "gray", Italic: true,
		}))
	case "title":
		if subtitle != "" {
			commands = append(commands, "minecraft:title "+target+" subtitle "+component(textComponent{Text: subtitle}))
		}
		commands = append(commands, "minecraft:title "+target+" title "+component(textComponent{Text: text}))
	case "actionbar":
		commands = append(commands, "minecraft:title "+target+" actionbar "+component(textComponent{Text: text}))
	default:
		return nil, httpapi.Errorf(http.StatusBadRequest, "Choose the chat, a title or the action bar.")
	}
	for _, c := range commands {
		if len(c) > maxCommand {
			return nil, httpapi.Errorf(http.StatusBadRequest, "The message is too long for the console. Shorten it.")
		}
	}
	return commands, nil
}

func validText(s string) bool {
	return utf8.RuneCountInString(s) <= maxMessage && !strings.ContainsFunc(s, unicode.IsControl)
}

func component(c textComponent) string {
	data, _ := json.Marshal(c) //nolint:errchkjson // strings and a bool always marshal
	return string(data)
}
