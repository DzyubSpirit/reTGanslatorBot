package config

import (
	"sort"
	"strings"
)

type Config struct {
	Chats        []Chat   `json:"chats"`
	HelpContacts []string `json:"help_contacts"`
}

type Chat struct {
	ID         int64    `json:"id"`
	Aliases    []string `json:"aliases"`
	ChildChats []Chat   `json:"child_chats"`
}

func (config Config) AllAliases() []string {
	aliases := make(map[string]bool)
	queue := config.Chats
	for len(queue) > 0 {
		chat := queue[0]
		queue = queue[1:]

		for _, alias := range chat.Aliases {
			aliases[alias] = true
		}
		queue = append(queue, chat.ChildChats...)
	}
	var aliasesList []string
	for alias := range aliases {
		aliasesList = append(aliasesList, alias)
	}
	sort.Slice(aliasesList, func(i, j int) bool {
		return strings.ToLower(aliasesList[i]) < strings.ToLower(aliasesList[j])
	})
	return aliasesList
}

func (config Config) AllChats() []Chat {
	var allChats []Chat
	queue := config.Chats
	for len(queue) > 0 {
		chat := queue[0]
		queue = queue[1:]

		allChats = append(allChats, chat)
		queue = append(queue, chat.ChildChats...)
	}
	return allChats
}
