package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// Создание публичного сервера
type CreateServerHandler struct {
	DB *sql.DB
}

type CreateServerRequest struct {
	QuestID    int    `json:"quest_id"`
	ServerName string `json:"server_name"`
	PlayerName string `json:"player_name"`
}

type CreateServerResponse struct {
	ServerID int `json:"server_id"`
}

func (h *CreateServerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req CreateServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Создаем сервер
	var serverID int
	err := h.DB.QueryRow(
		"INSERT INTO game_server (quest_id, server_name) VALUES ($1, $2) RETURNING id",
		req.QuestID, req.ServerName,
	).Scan(&serverID)
	if err != nil {
		http.Error(w, "Failed to create server", http.StatusInternalServerError)
		return
	}

	// Добавляем создателя как первого игрока
	_, err = h.DB.Exec(
		"INSERT INTO server_player (server_id, player_name) VALUES ($1, $2)",
		serverID, req.PlayerName,
	)
	if err != nil {
		http.Error(w, "Failed to add player to server", http.StatusInternalServerError)
		return
	}

	// Создаем мультиплеерное прохождение
	var initialStepID int
	err = h.DB.QueryRow("SELECT initial_step FROM quest WHERE id = $1", req.QuestID).Scan(&initialStepID)
	if err != nil {
		http.Error(w, "Quest not found", http.StatusNotFound)
		return
	}

	_, err = h.DB.Exec(
		"INSERT INTO multiplayer_playthrough (server_id, current_step) VALUES ($1, $2)",
		serverID, initialStepID,
	)
	if err != nil {
		http.Error(w, "Failed to create multiplayer playthrough", http.StatusInternalServerError)
		return
	}

	resp := CreateServerResponse{ServerID: serverID}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Получение списка серверов
type ListServersHandler struct {
	DB *sql.DB
}

type ServerInfo struct {
	ServerID    int    `json:"server_id"`
	ServerName  string `json:"server_name"`
	QuestTitle  string `json:"quest_title"`
	PlayerCount int    `json:"player_count"`
	MaxPlayers  int    `json:"max_players"`
	Status      string `json:"status"`
}

func (h *ListServersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(`
		SELECT gs.id, gs.server_name, q.title, COUNT(sp.id), gs.max_players, gs.status
		FROM game_server gs
		JOIN quest q ON gs.quest_id = q.id
		LEFT JOIN server_player sp ON gs.id = sp.server_id
		WHERE gs.is_public = true AND gs.status = 'waiting'
		GROUP BY gs.id, gs.server_name, q.title, gs.max_players, gs.status
		ORDER BY gs.created_at DESC
	`)
	if err != nil {
		http.Error(w, "Failed to get servers", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var servers []ServerInfo
	for rows.Next() {
		var server ServerInfo
		err := rows.Scan(&server.ServerID, &server.ServerName, &server.QuestTitle,
			&server.PlayerCount, &server.MaxPlayers, &server.Status)
		if err != nil {
			http.Error(w, "Failed to scan server", http.StatusInternalServerError)
			return
		}
		servers = append(servers, server)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(servers)
}

// Присоединение к серверу
type JoinServerHandler struct {
	DB *sql.DB
}

type JoinServerRequest struct {
	ServerID   int    `json:"server_id"`
	PlayerName string `json:"player_name"`
}

func (h *JoinServerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req JoinServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Проверяем, что сервер существует и есть место
	var playerCount, maxPlayers int
	var status string
	err := h.DB.QueryRow(`
		SELECT COUNT(sp.id), gs.max_players, gs.status
		FROM game_server gs
		LEFT JOIN server_player sp ON gs.id = sp.server_id
		WHERE gs.id = $1
		GROUP BY gs.max_players, gs.status
	`, req.ServerID).Scan(&playerCount, &maxPlayers, &status)
	if err != nil {
		http.Error(w, "Server not found", http.StatusNotFound)
		return
	}

	if status != "waiting" {
		http.Error(w, "Server is not available", http.StatusBadRequest)
		return
	}

	if playerCount >= maxPlayers {
		http.Error(w, "Server is full", http.StatusBadRequest)
		return
	}

	// Добавляем игрока
	_, err = h.DB.Exec(
		"INSERT INTO server_player (server_id, player_name) VALUES ($1, $2)",
		req.ServerID, req.PlayerName,
	)
	if err != nil {
		http.Error(w, "Failed to join server", http.StatusInternalServerError)
		return
	}

	// Если сервер заполнен, меняем статус
	if playerCount+1 >= maxPlayers {
		_, err = h.DB.Exec("UPDATE game_server SET status = 'in_progress' WHERE id = $1", req.ServerID)
		if err != nil {
			http.Error(w, "Failed to update server status", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "joined"})
}

// Получение текущего состояния мультиплеерной игры
type GetMultiplayerStateHandler struct {
	DB *sql.DB
}

type MultiplayerState struct {
	ServerID        int                         `json:"server_id"`
	CurrentStepID   int                         `json:"current_step_id"`
	StepText        string                      `json:"step_text,omitempty"`
	Choices         []PlayerActionChoiceProcess `json:"choices,omitempty"`
	PlayerChoices   map[string]int              `json:"player_choices,omitempty"`
	AllPlayersVoted bool                        `json:"all_players_voted"`
	NextStep        *int                        `json:"next_step,omitempty"`
}

// Получение диалога и вариантов ответа для всех игроков
type GetMultiplayerDialogHandler struct {
	DB *sql.DB
}

type PlayerChoice struct {
	PlayerName string                      `json:"player_name"`
	Choices    []PlayerActionChoiceProcess `json:"choices"`
	HasChosen  bool                        `json:"has_chosen"`
	ChosenID   int                         `json:"chosen_id,omitempty"`
}

type MultiplayerDialogState struct {
	ServerID      int            `json:"server_id"`
	CurrentStepID int            `json:"current_step_id"`
	StepText      string         `json:"step_text,omitempty"`
	StepType      string         `json:"step_type"`
	Players       []PlayerChoice `json:"players"`
	NextStep      *int           `json:"next_step,omitempty"`
}

type PlayerActionChoiceProcess struct {
	ChoiceID      int    `json:"choice_id"`
	Text          string `json:"text"`
	ViolencePoint int    `json:"violence_point"`
	WhateverPoint int    `json:"whatever_point"`
	PacifismPoint int    `json:"pacifism_point"`
	PlayerIndex   int    `json:"player_index"`
}

func (h *GetMultiplayerDialogHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	serverIDStr := r.URL.Query().Get("server_id")
	serverID, err := strconv.Atoi(serverIDStr)
	if err != nil {
		http.Error(w, "Invalid server_id", http.StatusBadRequest)
		return
	}

	// Получаем текущее состояние прохождения
	var currentStepID, violencePoint, whateverPoint, pacifismPoint int
	err = h.DB.QueryRow(`
		SELECT current_step, violence_point, whatever_point, pacifism_point
		FROM multiplayer_playthrough
		WHERE server_id = $1
	`, serverID).Scan(&currentStepID, &violencePoint, &whateverPoint, &pacifismPoint)
	if err != nil {
		http.Error(w, "Playthrough not found", http.StatusNotFound)
		return
	}

	// Получаем информацию о шаге
	var stepType string
	var stepText sql.NullString
	var nextStepID sql.NullInt64
	err = h.DB.QueryRow(`
		SELECT 
			CASE 
				WHEN na.id IS NOT NULL THEN 'narration'
				WHEN pa.id IS NOT NULL THEN 'player_action'
				WHEN ca.id IS NOT NULL THEN 'character_action'
			END as step_type,
			na.text,
			s.next_step
		FROM step s
		LEFT JOIN narration_action na ON s.id = na.step
		LEFT JOIN player_action pa ON s.id = pa.step
		LEFT JOIN character_action ca ON s.id = ca.step
		WHERE s.id = $1
	`, currentStepID).Scan(&stepType, &stepText, &nextStepID)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Failed to get step info", http.StatusInternalServerError)
		return
	}

	state := MultiplayerDialogState{
		ServerID:      serverID,
		CurrentStepID: currentStepID,
		StepType:      stepType,
	}

	if nextStepID.Valid {
		nextStep := int(nextStepID.Int64)
		state.NextStep = &nextStep
	}

	// Получаем список игроков
	playerRows, err := h.DB.Query(`
		SELECT player_name FROM server_player WHERE server_id = $1 ORDER BY joined_at
	`, serverID)
	if err != nil {
		http.Error(w, "Failed to get players", http.StatusInternalServerError)
		return
	}
	defer playerRows.Close()

	var playerNames []string
	for playerRows.Next() {
		var playerName string
		err := playerRows.Scan(&playerName)
		if err != nil {
			http.Error(w, "Failed to scan player name", http.StatusInternalServerError)
			return
		}
		playerNames = append(playerNames, playerName)
	}

	// Обработка в зависимости от типа шага
	if stepType == "narration" {
		state.StepText = stepText.String
		// Для нарративного шага у всех игроков одинаковые условия (нет выбора)
		for _, playerName := range playerNames {
			state.Players = append(state.Players, PlayerChoice{
				PlayerName: playerName,
				Choices:    []PlayerActionChoiceProcess{},
				HasChosen:  true,
			})
		}
	} else if stepType == "player_action" {
		// Получаем все варианты выбора для этого шага
		choiceRows, err := h.DB.Query(`
			SELECT pac.id, pac.text, pac.violence_point, pac.whatever_point, pac.pacifism_point, pac.player_index
			FROM player_action_choice pac
			JOIN player_action pa ON pac.player_action = pa.id
			WHERE pa.step = $1
		`, currentStepID)
		if err != nil {
			http.Error(w, "Failed to get choices", http.StatusInternalServerError)
			return
		}
		defer choiceRows.Close()

		playerChoicesMap := make(map[int][]PlayerActionChoiceProcess)
		for choiceRows.Next() {
			var choice PlayerActionChoiceProcess
			err := choiceRows.Scan(&choice.ChoiceID, &choice.Text, &choice.ViolencePoint, &choice.WhateverPoint, &choice.PacifismPoint, &choice.PlayerIndex)
			if err != nil {
				http.Error(w, "Failed to scan choice", http.StatusInternalServerError)
				return
			}
			playerChoicesMap[choice.PlayerIndex] = append(playerChoicesMap[choice.PlayerIndex], choice)
		}

		// Получаем выборы игроков
		playerChoiceRows, err := h.DB.Query(`
			SELECT pc.player_name, pc.choice_id
			FROM player_choice pc
			JOIN multiplayer_playthrough mp ON pc.multiplayer_playthrough = mp.id
			WHERE mp.server_id = $1 AND pc.step_id = $2
		`, serverID, currentStepID)
		if err != nil {
			http.Error(w, "Failed to get player choices", http.StatusInternalServerError)
			return
		}
		defer playerChoiceRows.Close()

		playerChoices := make(map[string]int)
		for playerChoiceRows.Next() {
			var playerName string
			var choiceID int
			err := playerChoiceRows.Scan(&playerName, &choiceID)
			if err != nil {
				http.Error(w, "Failed to scan player choice", http.StatusInternalServerError)
				return
			}
			playerChoices[playerName] = choiceID
		}

		// Формируем данные для каждого игрока
		for i, playerName := range playerNames {
			playerChoice := PlayerChoice{
				PlayerName: playerName,
				Choices:    playerChoicesMap[i+1], // Player index starts from 1
				HasChosen:  false,
			}

			if choiceID, exists := playerChoices[playerName]; exists {
				playerChoice.HasChosen = true
				playerChoice.ChosenID = choiceID
			}

			state.Players = append(state.Players, playerChoice)
		}

	} else if stepType == "character_action" {
		// Для character_action выбираем наиболее подходящий вариант в зависимости от очков
		var chosenText sql.NullString
		var chosenNextStep sql.NullInt64
		err := h.DB.QueryRow(`
			SELECT cac.text, cac.next_step
			FROM character_action_choice cac
			JOIN character_action ca ON cac.character_action = ca.id
			WHERE ca.step = $1
			ORDER BY 
				cac.priority DESC,
				ABS(cac.violence_point_condition - $2) + 
				ABS(cac.whatever_point_condition - $3) + 
				ABS(cac.pacifism_point_condition - $4) ASC
			LIMIT 1
		`, currentStepID, violencePoint, whateverPoint, pacifismPoint).Scan(&chosenText, &chosenNextStep)

		if err == nil {
			state.StepText = chosenText.String
			// Обновляем следующий шаг, если он указан в выборе
			if chosenNextStep.Valid {
				nextStep := int(chosenNextStep.Int64)
				state.NextStep = &nextStep
			}
		}

		// Для персонажных действий у всех игроков одинаковые условия (нет выбора)
		for _, playerName := range playerNames {
			state.Players = append(state.Players, PlayerChoice{
				PlayerName: playerName,
				Choices:    []PlayerActionChoiceProcess{},
				HasChosen:  true,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

func (h *GetMultiplayerStateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	serverIDStr := r.URL.Query().Get("server_id")
	serverID, err := strconv.Atoi(serverIDStr)
	if err != nil {
		http.Error(w, "Invalid server_id", http.StatusBadRequest)
		return
	}

	// Получаем текущее состояние прохождения
	var currentStepID, violencePoint, whateverPoint, pacifismPoint int
	err = h.DB.QueryRow(`
		SELECT current_step, violence_point, whatever_point, pacifism_point
		FROM multiplayer_playthrough
		WHERE server_id = $1
	`, serverID).Scan(&currentStepID, &violencePoint, &whateverPoint, &pacifismPoint)
	if err != nil {
		http.Error(w, "Playthrough not found", http.StatusNotFound)
		return
	}

	// Получаем информацию о шаге
	var stepType string
	var stepText sql.NullString
	var nextStepID sql.NullInt64
	err = h.DB.QueryRow(`
		SELECT 
			CASE 
				WHEN na.id IS NOT NULL THEN 'narration'
				WHEN pa.id IS NOT NULL THEN 'player_action'
				WHEN ca.id IS NOT NULL THEN 'character_action'
			END as step_type,
			na.text,
			s.next_step
		FROM step s
		LEFT JOIN narration_action na ON s.id = na.step
		LEFT JOIN player_action pa ON s.id = pa.step
		LEFT JOIN character_action ca ON s.id = ca.step
		WHERE s.id = $1
	`, currentStepID).Scan(&stepType, &stepText, &nextStepID)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Failed to get step info", http.StatusInternalServerError)
		return
	}

	state := MultiplayerState{
		ServerID:      serverID,
		CurrentStepID: currentStepID,
	}

	if nextStepID.Valid {
		nextStep := int(nextStepID.Int64)
		state.NextStep = &nextStep
	}

	// Обработка в зависимости от типа шага
	if stepType == "narration" {
		state.StepText = stepText.String
	} else if stepType == "player_action" {
		// Получаем варианты выбора
		rows, err := h.DB.Query(`
			SELECT pac.id, pac.text, pac.violence_point, pac.whatever_point, pac.pacifism_point
			FROM player_action_choice pac
			JOIN player_action pa ON pac.player_action = pa.id
			WHERE pa.step = $1
		`, currentStepID)
		if err != nil {
			http.Error(w, "Failed to get choices", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var choices []PlayerActionChoiceProcess
		for rows.Next() {
			var choice PlayerActionChoiceProcess
			err := rows.Scan(&choice.ChoiceID, &choice.Text, &choice.ViolencePoint, &choice.WhateverPoint, &choice.PacifismPoint)
			if err != nil {
				http.Error(w, "Failed to scan choice", http.StatusInternalServerError)
				return
			}
			choices = append(choices, choice)
		}
		state.Choices = choices

		// Получаем выборы игроков
		choiceRows, err := h.DB.Query(`
			SELECT pc.player_name, pc.choice_id
			FROM player_choice pc
			JOIN multiplayer_playthrough mp ON pc.multiplayer_playthrough = mp.id
			WHERE mp.server_id = $1 AND pc.step_id = $2
		`, serverID, currentStepID)
		if err != nil {
			http.Error(w, "Failed to get player choices", http.StatusInternalServerError)
			return
		}
		defer choiceRows.Close()

		playerChoices := make(map[string]int)
		for choiceRows.Next() {
			var playerName string
			var choiceID int
			err := choiceRows.Scan(&playerName, &choiceID)
			if err != nil {
				http.Error(w, "Failed to scan player choice", http.StatusInternalServerError)
				return
			}
			playerChoices[playerName] = choiceID
		}
		state.PlayerChoices = playerChoices

		// Проверяем, проголосовали ли все игроки
		var playerCount int
		h.DB.QueryRow("SELECT COUNT(*) FROM server_player WHERE server_id = $1", serverID).Scan(&playerCount)
		state.AllPlayersVoted = len(playerChoices) == playerCount

	} else if stepType == "character_action" {
		// Для character_action выбираем наиболее подходящий вариант в зависимости от очков
		err := h.DB.QueryRow(`
			SELECT cac.text
			FROM character_action_choice cac
			JOIN character_action ca ON cac.character_action = ca.id
			WHERE ca.step = $1
			ORDER BY 
				cac.priority DESC,
				ABS(cac.violence_point_condition - $2) + 
				ABS(cac.whatever_point_condition - $3) + 
				ABS(cac.pacifism_point_condition - $4) ASC
			LIMIT 1
		`, currentStepID, violencePoint, whateverPoint, pacifismPoint).Scan(&stepText)
		if err == nil {
			state.StepText = stepText.String
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

// Совершение выбора в мультиплеере
type MakeMultiplayerChoiceHandler struct {
	DB *sql.DB
}

type MultiplayerChoiceRequest struct {
	ServerID   int    `json:"server_id"`
	PlayerName string `json:"player_name"`
	ChoiceID   int    `json:"choice_id"`
}

func (h *MakeMultiplayerChoiceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req MultiplayerChoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Получаем ID прохождения и текущий шаг
	var playthroughID, currentStepID int
	err := h.DB.QueryRow(`
		SELECT id, current_step
		FROM multiplayer_playthrough
		WHERE server_id = $1
	`, req.ServerID).Scan(&playthroughID, &currentStepID)
	if err != nil {
		http.Error(w, "Playthrough not found", http.StatusNotFound)
		return
	}

	// Сохраняем выбор игрока
	_, err = h.DB.Exec(`
		INSERT INTO player_choice (multiplayer_playthrough, player_name, choice_id, step_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (multiplayer_playthrough, player_name, step_id) 
		DO UPDATE SET choice_id = EXCLUDED.choice_id
	`, playthroughID, req.PlayerName, req.ChoiceID, currentStepID)
	if err != nil {
		http.Error(w, "Failed to save choice", http.StatusInternalServerError)
		return
	}

	// Проверяем, проголосовали ли все игроки
	var playerCount, choiceCount int
	h.DB.QueryRow("SELECT COUNT(*) FROM server_player WHERE server_id = $1", req.ServerID).Scan(&playerCount)
	h.DB.QueryRow(`
		SELECT COUNT(*) FROM player_choice pc
		JOIN multiplayer_playthrough mp ON pc.multiplayer_playthrough = mp.id
		WHERE mp.server_id = $1 AND pc.step_id = $2
	`, req.ServerID, currentStepID).Scan(&choiceCount)

	// Если все проголосовали, обрабатываем результат
	if choiceCount == playerCount {
		// Получаем следующий шаг и суммируем очки от всех выборов
		var totalViolence, totalWhatever, totalPacifism int
		var _ sql.NullInt64
		rows, err := h.DB.Query(`
			SELECT pac.violence_point, pac.whatever_point, pac.pacifism_point, s.next_step
			FROM player_choice pc
			JOIN multiplayer_playthrough mp ON pc.multiplayer_playthrough = mp.id
			JOIN player_action_choice pac ON pc.choice_id = pac.id
			JOIN player_action pa ON pac.player_action = pa.id
			JOIN step s ON pa.step = s.id
			WHERE mp.server_id = $1 AND pc.step_id = $2
		`, req.ServerID, currentStepID)
		if err != nil {
			http.Error(w, "Failed to get choice points", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var calculatedNextStepID int64 = 0

		for rows.Next() {
			var violence, whatever, pacifism int
			var nextStep sql.NullInt64
			err := rows.Scan(&violence, &whatever, &pacifism, &nextStep)
			if err != nil {
				http.Error(w, "Failed to scan points", http.StatusInternalServerError)
				return
			}
			totalViolence += violence
			totalWhatever += whatever
			totalPacifism += pacifism
			if nextStep.Valid {
				calculatedNextStepID = nextStep.Int64 // Все выборы должны вести к одному следующему шагу
			}

		}

		// Проверяем, есть ли следующий шаг (квест завершен?)
		if calculatedNextStepID == 0 {
			// Квест завершен, обновляем статус сервера
			_, err = h.DB.Exec(`
				UPDATE game_server SET status = 'finished' WHERE id = $1
			`, req.ServerID)
			if err != nil {
				http.Error(w, "Failed to finish game", http.StatusInternalServerError)
				return
			}
		}

		// Обновляем прохождение
		_, err = h.DB.Exec(`
			UPDATE multiplayer_playthrough
			SET current_step = $1,
				violence_point = violence_point + $2,
				whatever_point = whatever_point + $3,
				pacifism_point = pacifism_point + $4
			WHERE server_id = $5
		`, calculatedNextStepID, totalViolence, totalWhatever, totalPacifism, req.ServerID)
		if err != nil {
			http.Error(w, "Failed to update playthrough", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "choice_saved"})
}

// Переход к следующему шагу без выбора (для narration и character_action)
type ProceedToNextStepHandler struct {
	DB *sql.DB
}

type ProceedRequest struct {
	ServerID int `json:"server_id"`
}

func (h *ProceedToNextStepHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req ProceedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Получаем текущее состояние и очки
	var currentStepID, violencePoint, whateverPoint, pacifismPoint int
	var nextStepID sql.NullInt64
	err := h.DB.QueryRow(`
		SELECT mp.current_step, mp.violence_point, mp.whatever_point, mp.pacifism_point, s.next_step
		FROM multiplayer_playthrough mp
		JOIN step s ON mp.current_step = s.id
		WHERE mp.server_id = $1
	`, req.ServerID).Scan(&currentStepID, &violencePoint, &whateverPoint, &pacifismPoint, &nextStepID)
	if err != nil {
		http.Error(w, "Playthrough not found", http.StatusNotFound)
		return
	}

	// Проверяем тип шага - можно переходить только для narration и character_action
	var stepType string
	err = h.DB.QueryRow(`
		SELECT 
			CASE 
				WHEN na.id IS NOT NULL THEN 'narration'
				WHEN pa.id IS NOT NULL THEN 'player_action'
				WHEN ca.id IS NOT NULL THEN 'character_action'
			END as step_type
		FROM step s
		LEFT JOIN narration_action na ON s.id = na.step
		LEFT JOIN player_action pa ON s.id = pa.step
		LEFT JOIN character_action ca ON s.id = ca.step
		WHERE s.id = $1
	`, currentStepID).Scan(&stepType)
	if err != nil {
		http.Error(w, "Failed to get step type", http.StatusInternalServerError)
		return
	}

	if stepType == "player_action" {
		http.Error(w, "Cannot proceed without player choice", http.StatusBadRequest)
		return
	}

	// Для character_action проверяем, есть ли специфичный следующий шаг
	if stepType == "character_action" {
		var characterNextStep sql.NullInt64
		err = h.DB.QueryRow(`
			SELECT cac.next_step
			FROM character_action_choice cac
			JOIN character_action ca ON cac.character_action = ca.id
			WHERE ca.step = $1
			ORDER BY 
				cac.priority DESC,
				ABS(cac.violence_point_condition - $2) + 
				ABS(cac.whatever_point_condition - $3) + 
				ABS(cac.pacifism_point_condition - $4) ASC
			LIMIT 1
		`, currentStepID, violencePoint, whateverPoint, pacifismPoint).Scan(&characterNextStep)

		if err == nil && characterNextStep.Valid {
			nextStepID = characterNextStep
		}
	}

	if !nextStepID.Valid {
		// Квест завершен
		_, err = h.DB.Exec(`
			UPDATE game_server SET status = 'finished' WHERE id = $1
		`, req.ServerID)
		if err != nil {
			http.Error(w, "Failed to finish game", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "quest_finished"})
		return
	}

	// Переходим к следующему шагу
	_, err = h.DB.Exec(`
		UPDATE multiplayer_playthrough
		SET current_step = $1
		WHERE server_id = $2
	`, nextStepID.Int64, req.ServerID)
	if err != nil {
		http.Error(w, "Failed to proceed to next step", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "proceeded"})
}
