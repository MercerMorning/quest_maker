package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
)

type GetCurrentStepHandler struct {
	DB *sql.DB
}

type MakeChoiceHandler struct {
	DB *sql.DB
}

type StepResponse struct {
	StepID   int                         `json:"step_id"`
	Text     string                      `json:"text"`
	Choices  []PlayerActionChoiceProcess `json:"choices,omitempty"`
	NextStep int                         `json:"next_step,omitempty"`
}

func (h *GetCurrentStepHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	playthroughID := r.URL.Query().Get("playthrough_id")
	if playthroughID == "" {
		http.Error(w, "Missing playthrough_id", http.StatusBadRequest)
		return
	}

	var currentStepID, nextStepID, violencePoint, whateverPoint, pacifismPoint int
	var stepText sql.NullString
	var stepType string // narration, player_action, character_action

	err := h.DB.QueryRow(`
		SELECT s.id, s.next_step, p.violence_point, p.whatever_point, p.pacifism_point,
		       COALESCE(na.text, NULL) AS narration_text,
		       CASE 
		           WHEN pa.step IS NOT NULL THEN 'player_action'
		           WHEN ca.step IS NOT NULL THEN 'character_action'
		           ELSE 'narration'
		       END AS step_type
		FROM playthrough p
		JOIN step s ON p.step = s.id
		LEFT JOIN narration_action na ON s.id = na.step
		LEFT JOIN player_action pa ON s.id = pa.step
		LEFT JOIN character_action ca ON s.id = ca.step
		WHERE p.id = $1`, playthroughID).Scan(&currentStepID, &nextStepID, &violencePoint, &whateverPoint, &pacifismPoint, &stepText, &stepType)

	if err != nil {
		http.Error(w, "Failed to get current step", http.StatusInternalServerError)
		return
	}

	var choices []PlayerActionChoiceProcess

	if stepType == "player_action" {
		// Для player_action выбор идёт из player_action_choice
		rows, err := h.DB.Query(`
			SELECT pac.id, pac.text, pac.violence_point, pac.whatever_point, pac.pacifism_point
			FROM player_action_choice pac
			JOIN player_action pa ON pac.player_action = pa.id
			WHERE pa.step = $1`, currentStepID)
		if err != nil {
			http.Error(w, "Failed to get player action choices", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var choice PlayerActionChoiceProcess
			if err := rows.Scan(&choice.ChoiceID, &choice.Text, &choice.ViolencePoint, &choice.WhateverPoint, &choice.PacifismPoint); err != nil {
				http.Error(w, "Failed to scan player action choice", http.StatusInternalServerError)
				return
			}
			choices = append(choices, choice)
		}
		// Текст не выводим
		stepText.Valid = false
	}

	if stepType == "character_action" {
		// Для character_action текст выбираем в зависимости от текущих показателей
		err := h.DB.QueryRow(`
			SELECT cac.text
			FROM character_action_choice cac
			JOIN character_action ca ON cac.character_action = ca.id
			WHERE ca.step = $1
			ORDER BY 
				ABS(cac.violence_point_condition - $2) + 
				ABS(cac.whatever_point_condition - $3) + 
				ABS(cac.pacifism_point_condition - $4)
			LIMIT 1`, currentStepID, violencePoint, whateverPoint, pacifismPoint).Scan(&stepText)

		if err != nil {
			http.Error(w, "Failed to get character action text", http.StatusInternalServerError)
			return
		}
	}

	// Если выбора нет и есть следующий шаг, обновляем `playthrough`
	if len(choices) == 0 && nextStepID != 0 {
		_, err := h.DB.Exec(`UPDATE playthrough SET step = $1 WHERE id = $2`, nextStepID, playthroughID)
		if err != nil {
			http.Error(w, "Failed to update step", http.StatusInternalServerError)
			return
		}
	}

	// Формируем ответ
	response := StepResponse{
		StepID:   currentStepID,
		Text:     "",
		Choices:  choices,
		NextStep: nextStepID,
	}

	if stepText.Valid {
		response.Text = stepText.String
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// Структура для запроса
type ChoiceRequest struct {
	PlaythroughID int `json:"playthrough_id"`
	ChoiceID      int `json:"choice_id"`
}

// Обработчик выбора действия
func (h *MakeChoiceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req ChoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Получаем ID следующего шага и изменения очков, основываясь на выбранном действии
	var nextStepID, violencePoint, whateverPoint, pacifismPoint int
	err := h.DB.QueryRow(`
		SELECT s.next_step, pac.violence_point, pac.whatever_point, pac.pacifism_point
		FROM player_action_choice pac
        JOIN player_action pa ON pac.player_action = pa.id
        JOIN step s ON pa.step = s.id
		WHERE pac.id = $1
`, req.ChoiceID).Scan(&nextStepID, &violencePoint, &whateverPoint, &pacifismPoint)
	if err != nil {
		http.Error(w, "Failed to get next step and points", http.StatusInternalServerError)
		return
	}

	// Обновляем очки в playthrough, в зависимости от выбора
	_, err = h.DB.Exec(`
		UPDATE playthrough
		SET step = $1,
			violence_point = violence_point + $2,
			whatever_point = whatever_point + $3,
			pacifism_point = pacifism_point + $4
		WHERE id = $5`, nextStepID, violencePoint, whateverPoint, pacifismPoint, req.PlaythroughID)
	if err != nil {
		http.Error(w, "Failed to update playthrough", http.StatusInternalServerError)
		return
	}
	fmt.Println("popaa")
	fmt.Println(nextStepID)
	fmt.Println(req.PlaythroughID)

	// Ответ с текстом следующего шага и его ID
	response := StepResponse{
		StepID:   nextStepID,
		Text:     "",
		NextStep: nextStepID,
	}

	// Отправляем успешный ответ
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
