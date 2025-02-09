package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

type QuestRequest struct {
	Title      string         `json:"title"`
	Characters []CharacterReq `json:"characters"`
	Steps      []StepReq      `json:"steps"`
}

type CharacterReq struct {
	Name string `json:"name"`
}

type StepReq struct {
	Type string      `json:"type"`
	Body interface{} `json:"body"`
}

type NarrationBody struct {
	Text string `json:"text"`
}

type PlayerActionBody struct {
	Choices []PlayerActionChoice `json:"choices"`
}

type PlayerActionChoice struct {
	Text          string `json:"text"`
	ViolencePoint int    `json:"violence_point"`
	WhateverPoint int    `json:"whatever_point"`
	PacifismPoint int    `json:"pacifism_point"`
}

type CharacterActionBody struct {
	CharacterName          string `json:"character_name"`
	Text                   string `json:"text"`
	ViolencePointCondition int    `json:"violence_point_condition"`
	WhateverPointCondition int    `json:"whatever_point_condition"`
	PacifismPointCondition int    `json:"pacifism_point_condition"`
}

type MakeQuestHandler struct {
	DB *sql.DB
}

func (h *MakeQuestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req QuestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	tx, err := h.DB.Begin()
	if err != nil {
		http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	characterIDs := make(map[string]int)
	var questID int
	err = tx.QueryRow("INSERT INTO quest (title, created_at, updated_at) VALUES ($1, NOW(), NOW()) RETURNING id", req.Title).Scan(&questID)
	if err != nil {
		http.Error(w, "Failed to insert quest", http.StatusInternalServerError)
		return
	}

	for _, char := range req.Characters {
		var charID int
		err := tx.QueryRow("INSERT INTO character (quest, name) VALUES ($1, $2) RETURNING id", questID, char.Name).Scan(&charID)
		if err != nil {
			http.Error(w, "Failed to insert character", http.StatusInternalServerError)
			return
		}
		characterIDs[char.Name] = charID
	}

	var firstStepID int
	var prevStepID *int
	for i, step := range req.Steps {
		var stepID int
		err := tx.QueryRow("INSERT INTO step (number, created_at, updated_at) VALUES ($1, NOW(), NOW()) RETURNING id", i+1).Scan(&stepID)
		if err != nil {
			http.Error(w, "Failed to insert step", http.StatusInternalServerError)
			return
		}

		// Устанавливаем связь с предыдущим шагом
		if prevStepID != nil {
			_, err := tx.Exec("UPDATE step SET next_step = $1 WHERE id = $2", stepID, *prevStepID)
			if err != nil {
				http.Error(w, "Failed to update next_step", http.StatusInternalServerError)
				return
			}
		}

		switch step.Type {
		case "narration":
			body := step.Body.(map[string]interface{})
			_, err := tx.Exec("INSERT INTO narration_action (step, text) VALUES ($1, $2)", stepID, body["text"].(string))
			if err != nil {
				http.Error(w, "Failed to insert narration action", http.StatusInternalServerError)
				return
			}

		case "player_action":
			var playerActionID int
			err := tx.QueryRow("INSERT INTO player_action (step) VALUES ($1) RETURNING id", stepID).Scan(&playerActionID)
			if err != nil {
				http.Error(w, "Failed to insert player action", http.StatusInternalServerError)
				return
			}

			body := step.Body.(map[string]interface{})
			choices := body["choices"].([]interface{})
			for _, choice := range choices {
				c := choice.(map[string]interface{})
				_, err := tx.Exec(
					"INSERT INTO player_action_choice (player_action, text, violence_point, whatever_point, pacifism_point) VALUES ($1, $2, $3, $4, $5)",
					playerActionID, c["text"].(string),
					int(c["violence_point"].(float64)), int(c["whatever_point"].(float64)), int(c["pacifism_point"].(float64)),
				)
				if err != nil {
					http.Error(w, "Failed to insert player action choice", http.StatusInternalServerError)
					return
				}
			}

		case "character_action":
			body := step.Body.(map[string]interface{})
			charID, exists := characterIDs[body["character_name"].(string)]
			if !exists {
				http.Error(w, "Character not found", http.StatusBadRequest)
				return
			}

			var characterActionID int
			err := tx.QueryRow("INSERT INTO character_action (character, step) VALUES ($1, $2) RETURNING id", charID, stepID).Scan(&characterActionID)
			if err != nil {
				http.Error(w, "Failed to insert character action", http.StatusInternalServerError)
				return
			}

			_, err = tx.Exec(
				"INSERT INTO character_action_choice (character_action, text, violence_point_condition, whatever_point_condition, pacifism_point_condition) VALUES ($1, $2, $3, $4, $5)",
				characterActionID, body["text"].(string),
				int(body["violence_point_condition"].(float64)), int(body["whatever_point_condition"].(float64)), int(body["pacifism_point_condition"].(float64)),
			)
			if err != nil {
				http.Error(w, "Failed to insert character action choice", http.StatusInternalServerError)
				return
			}
		}
		// Сохраняем ID текущего шага для следующей итерации
		prevStepID = &stepID

		if i == 0 {
			firstStepID = stepID
		}
	}

	_, err = tx.Exec("UPDATE quest SET initial_step = $1 WHERE id = $2", firstStepID, questID)
	if err != nil {
		http.Error(w, "Failed to update quest initial_step", http.StatusInternalServerError)
		return
	}

	tx.Commit()
	w.WriteHeader(http.StatusCreated)
}
