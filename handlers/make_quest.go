package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
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
	PlayerIndex   int    `json:"player_index"` // 1 для первого игрока, 2 для второго
}

type CharacterActionBody struct {
	CharacterName string                  `json:"character_name"`
	Choices       []CharacterActionChoice `json:"choices"`
}

type CharacterActionChoice struct {
	Text                   string `json:"text"`
	ViolencePointCondition int    `json:"violence_point_condition"`
	WhateverPointCondition int    `json:"whatever_point_condition"`
	PacifismPointCondition int    `json:"pacifism_point_condition"`
	Priority               int    `json:"priority"`
	NextStepNumber         int    `json:"next_step_number"` // Номер шага для перехода
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
		fmt.Println(err)
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

	// Сначала создаем все шаги без содержимого
	stepIDs := make([]int, len(req.Steps))
	for i := range req.Steps {
		var stepID int
		err := tx.QueryRow("INSERT INTO step (number, created_at, updated_at) VALUES ($1, NOW(), NOW()) RETURNING id", i+1).Scan(&stepID)
		if err != nil {
			http.Error(w, "Failed to insert step", http.StatusInternalServerError)
			return
		}
		stepIDs[i] = stepID
	}

	// Устанавливаем связи между шагами (каждый шаг ведет к следующему по умолчанию)
	for i := 0; i < len(stepIDs)-1; i++ {
		_, err := tx.Exec("UPDATE step SET next_step = $1 WHERE id = $2", stepIDs[i+1], stepIDs[i])
		if err != nil {
			http.Error(w, "Failed to update next_step", http.StatusInternalServerError)
			return
		}
	}

	// Теперь создаем содержимое для каждого шага
	for i, step := range req.Steps {
		stepID := stepIDs[i]

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
				playerIndex := 0 // По умолчанию для всех игроков
				if idx, ok := c["player_index"]; ok {
					playerIndex = int(idx.(float64))
				}
				_, err := tx.Exec(
					"INSERT INTO player_action_choice (player_action, text, violence_point, whatever_point, pacifism_point, player_index) VALUES ($1, $2, $3, $4, $5, $6)",
					playerActionID, c["text"].(string),
					int(c["violence_point"].(float64)), int(c["whatever_point"].(float64)), int(c["pacifism_point"].(float64)),
					playerIndex,
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

			choicesInterface, exists := body["choices"]
			if !exists || choicesInterface == nil {
				http.Error(w, "Missing or invalid choices in character_action", http.StatusBadRequest)
				return
			}

			choices, ok := choicesInterface.([]interface{})
			if !ok {
				http.Error(w, "Choices must be an array", http.StatusBadRequest)
				return
			}

			for _, choice := range choices {
				c := choice.(map[string]interface{})

				// Находим ID следующего шага по его номеру
				var nextStepID sql.NullInt64
				if nextStepNum, ok := c["next_step_number"]; ok && nextStepNum != nil {
					stepNum := int(nextStepNum.(float64))
					// Проверяем, что номер шага валиден (от 1 до количества шагов)
					if stepNum > 0 && stepNum <= len(req.Steps) {
						// Используем правильный ID шага из массива (stepNum-1 так как массив с нуля)
						nextStepID.Valid = true
						nextStepID.Int64 = int64(stepIDs[stepNum-1])
					}
				}

				priority := 0
				if p, ok := c["priority"]; ok {
					priority = int(p.(float64))
				}

				_, err = tx.Exec(
					"INSERT INTO character_action_choice (character_action, text, violence_point_condition, whatever_point_condition, pacifism_point_condition, priority, next_step) VALUES ($1, $2, $3, $4, $5, $6, $7)",
					characterActionID, c["text"].(string),
					int(c["violence_point_condition"].(float64)),
					int(c["whatever_point_condition"].(float64)),
					int(c["pacifism_point_condition"].(float64)),
					priority,
					nextStepID,
				)
				if err != nil {
					http.Error(w, "Failed to insert character action choice", http.StatusInternalServerError)
					return
				}
			}
		}
	}

	// Устанавливаем начальный шаг квеста
	_, err = tx.Exec("UPDATE quest SET initial_step = $1 WHERE id = $2", stepIDs[0], questID)
	if err != nil {
		http.Error(w, "Failed to update quest initial_step", http.StatusInternalServerError)
		return
	}

	tx.Commit()
	w.WriteHeader(http.StatusCreated)
}
