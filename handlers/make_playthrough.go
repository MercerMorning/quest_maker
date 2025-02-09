package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

type StartPlaythroughRequest struct {
	PlayerName string `json:"player_name"`
	QuestID    int    `json:"quest_id"`
}

type StartPlaythroughResponse struct {
	PlaythroughID int `json:"playthrough_id"`
}

type MakePlayThroughHandler struct {
	DB *sql.DB
}

func (h *MakePlayThroughHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req StartPlaythroughRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Найдем начальный шаг квеста
	var initialStepID int
	err := h.DB.QueryRow("SELECT initial_step FROM quest WHERE id = $1", req.QuestID).Scan(&initialStepID)
	if err != nil {
		http.Error(w, "Quest not found", http.StatusNotFound)
		return
	}

	// Создаем запись о прохождении
	var playthroughID int
	err = h.DB.QueryRow(
		"INSERT INTO playthrough (player_name, quest, step) VALUES ($1, $2, $3) RETURNING id",
		req.PlayerName, req.QuestID, initialStepID,
	).Scan(&playthroughID)
	if err != nil {
		http.Error(w, "Failed to start playthrough", http.StatusInternalServerError)
		return
	}

	// Отправляем ID нового прохождения
	resp := StartPlaythroughResponse{PlaythroughID: playthroughID}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
