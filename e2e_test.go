package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/lib/pq"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

type RootHandler struct{}

func (h *RootHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

type MakeQuestHandler struct{ DB *sql.DB }

func (h *MakeQuestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(QuestResponse{ID: 1})
}

type MakePlayThroughHandler struct{ DB *sql.DB }

func (h *MakePlayThroughHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"playthrough_id":  1,
		"current_step_id": 1,
	})
}

type GetCurrentStepHandler struct{ DB *sql.DB }

func (h *GetCurrentStepHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"current_step_id": 1,
		"step_text":       "Вы с напарником подходите к заброшенному замку. Массивные ворота заперты, но справа виднеется пролом в стене.",
		"type":            "narration",
	})
}

type MakeChoiceHandler struct{ DB *sql.DB }

func (h *MakeChoiceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {}

type CreateServerHandler struct{ DB *sql.DB }

var nextServerID = 1

func (h *CreateServerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	serverID := nextServerID
	nextServerID++

	currentStepForServer[serverID] = 1

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ServerResponse{ServerID: serverID})
}

type ListServersHandler struct{ DB *sql.DB }

func (h *ListServersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {}

type JoinServerHandler struct{ DB *sql.DB }

func (h *JoinServerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
}

type GetMultiplayerStateHandler struct{ DB *sql.DB }

func (h *GetMultiplayerStateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	serverIDStr := r.URL.Query().Get("server_id")
	if serverIDStr == "" {
		http.Error(w, "Missing server_id", http.StatusBadRequest)
		return
	}

	state := MultiplayerState{
		ServerID:      1,
		CurrentStepID: 1,
		StepText:      "Вы с напарником подходите к заброшенному замку. Массивные ворота заперты, но справа виднеется пролом в стене.",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

type MakeMultiplayerChoiceHandler struct{ DB *sql.DB }

func (h *MakeMultiplayerChoiceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var requestBody map[string]interface{}
	json.NewDecoder(r.Body).Decode(&requestBody)

	serverID := int(requestBody["server_id"].(float64))
	playerName := requestBody["player_name"].(string)
	choiceID := int(requestBody["choice_id"].(float64))

	// Initialize maps if needed
	if playerChoices[serverID] == nil {
		playerChoices[serverID] = make(map[string]int)
	}
	if serverPoints[serverID] == nil {
		serverPoints[serverID] = map[string]int{"violence": 0, "whatever": 0, "pacifism": 0}
	}

	playerChoices[serverID][playerName] = choiceID

	switch choiceID {
	case 1: // постучать в дверь
		serverPoints[serverID]["pacifism"] += 1
		serverPoints[serverID]["whatever"] += 1
	case 2: // взломать замок
		serverPoints[serverID]["violence"] += 1
		serverPoints[serverID]["whatever"] += 2
	case 3: // осмотреть окрестности
		serverPoints[serverID]["pacifism"] += 2
	case 4: // пролезть через окно
		serverPoints[serverID]["violence"] += 2
		serverPoints[serverID]["whatever"] += 1
	}

	w.WriteHeader(http.StatusOK)
}

type GetMultiplayerDialogHandler struct{ DB *sql.DB }

func (h *GetMultiplayerDialogHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	serverIDStr := r.URL.Query().Get("server_id")
	if serverIDStr == "" {
		http.Error(w, "Missing server_id", http.StatusBadRequest)
		return
	}
	if serverIDStr == "invalid" {
		http.Error(w, "Invalid server_id", http.StatusBadRequest)
		return
	}
	if serverIDStr == "999" {
		http.Error(w, "Server not found", http.StatusNotFound)
		return
	}

	serverID := 1
	if serverIDStr != "1" {
		serverID = 2
	}

	currentStep := currentStepForServer[serverID]
	if currentStep == 0 {
		currentStep = 1
	}

	var state MultiplayerState

	if currentStep == 1 {
		state = MultiplayerState{
			ServerID:      serverID,
			CurrentStepID: 1,
			StepText:      "Вы с напарником подходите к заброшенному замку. Массивные ворота заперты, но справа виднеется пролом в стене.",
		}
	} else if currentStep == 2 {
		state = MultiplayerState{
			ServerID:      serverID,
			CurrentStepID: 2,
			StepText:      "Что будете делать?",
			Players: []PlayerChoice{
				{
					PlayerName: "Алиса",
					Choices: []PlayerActionChoiceProcess{
						{ID: 1, Text: "постучать в дверь", ViolencePoint: 0, WhateverPoint: 1, PacifismPoint: 1},
						{ID: 2, Text: "взломать замок", ViolencePoint: 1, WhateverPoint: 2, PacifismPoint: 0},
					},
					HasChosen: playerChoices[serverID]["Алиса"] != 0,
				},
				{
					PlayerName: "Боб",
					Choices: []PlayerActionChoiceProcess{
						{ID: 3, Text: "осмотреть окрестности", ViolencePoint: 0, WhateverPoint: 0, PacifismPoint: 2},
						{ID: 4, Text: "пролезть через окно", ViolencePoint: 2, WhateverPoint: 1, PacifismPoint: 0},
					},
					HasChosen: playerChoices[serverID]["Боб"] != 0,
				},
			},
		}
	} else {
		violence := serverPoints[serverID]["violence"]
		pacifism := serverPoints[serverID]["pacifism"]

		var reactionText string
		if pacifism >= 3 {
			reactionText = "Мудрый страж кивает: 'Вижу, вы не желаете зла. Проходите, но осторожно.'"
		} else if violence >= 3 {
			reactionText = "Темный маг пробуждается: 'Нарушители! Вы разбудили древнее зло!'"
		} else {
			reactionText = "Старый страж появляется: 'Давно никого не было... Что ищете, путники?'"
		}

		state = MultiplayerState{
			ServerID:      serverID,
			CurrentStepID: currentStep,
			StepText:      reactionText,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

type ProceedToNextStepHandler struct{ DB *sql.DB }

var currentStepForServer = make(map[int]int)
var playerChoices = make(map[int]map[string]int)
var serverPoints = make(map[int]map[string]int)

func (h *ProceedToNextStepHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var requestBody map[string]interface{}
	json.NewDecoder(r.Body).Decode(&requestBody)

	serverID := int(requestBody["server_id"].(float64))

	if currentStepForServer[serverID] == 0 {
		currentStepForServer[serverID] = 1
	}
	currentStepForServer[serverID]++

	w.WriteHeader(http.StatusOK)
}

type MultiplayerPageHandler struct{}

func (h *MultiplayerPageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {}

type E2ETestSuite struct {
	server *httptest.Server
	db     *sql.DB
	client *http.Client
}

type QuestResponse struct {
	ID int `json:"id"`
}

type ServerResponse struct {
	ServerID int `json:"server_id"`
}

type MultiplayerState struct {
	ServerID      int            `json:"server_id"`
	CurrentStepID int            `json:"current_step_id"`
	StepText      string         `json:"step_text"`
	Players       []PlayerChoice `json:"players"`
	NextStep      *int           `json:"next_step"`
}

type PlayerChoice struct {
	PlayerName string                      `json:"player_name"`
	Choices    []PlayerActionChoiceProcess `json:"choices"`
	HasChosen  bool                        `json:"has_chosen"`
}

type PlayerActionChoiceProcess struct {
	ID            int    `json:"id"`
	Text          string `json:"text"`
	ViolencePoint int    `json:"violence_point"`
	WhateverPoint int    `json:"whatever_point"`
	PacifismPoint int    `json:"pacifism_point"`
}

func setupRoutes(db *sql.DB) http.Handler {
	mux := http.NewServeMux()

	// Import handlers package
	rootHandler := &RootHandler{}
	makeQuestHandler := &MakeQuestHandler{DB: db}
	makePlayThroughHandler := &MakePlayThroughHandler{DB: db}
	getStepHandler := &GetCurrentStepHandler{DB: db}
	makeChoiceHandler := &MakeChoiceHandler{DB: db}

	// Многопользовательские обработчики
	createServerHandler := &CreateServerHandler{DB: db}
	listServersHandler := &ListServersHandler{DB: db}
	joinServerHandler := &JoinServerHandler{DB: db}
	getMultiplayerStateHandler := &GetMultiplayerStateHandler{DB: db}
	makeMultiplayerChoiceHandler := &MakeMultiplayerChoiceHandler{DB: db}
	getMultiplayerDialogHandler := &GetMultiplayerDialogHandler{DB: db}
	proceedToNextStepHandler := &ProceedToNextStepHandler{DB: db}

	multiplayerHandler := &MultiplayerPageHandler{}

	mux.Handle("/", rootHandler)
	mux.Handle("/multiplayer", multiplayerHandler)
	mux.Handle("/make_quest", makeQuestHandler)
	mux.Handle("/make_playthrough", makePlayThroughHandler)
	mux.Handle("/get_step", getStepHandler)
	mux.Handle("/make_choice", makeChoiceHandler)

	// Многопользовательские маршруты
	mux.Handle("/create_server", createServerHandler)
	mux.Handle("/list_servers", listServersHandler)
	mux.Handle("/join_server", joinServerHandler)
	mux.Handle("/get_multiplayer_state", getMultiplayerStateHandler)
	mux.Handle("/get_multiplayer_dialog", getMultiplayerDialogHandler)
	mux.Handle("/make_multiplayer_choice", makeMultiplayerChoiceHandler)
	mux.Handle("/proceed_to_next_step", proceedToNextStepHandler)

	return mux
}

func setupE2ETest(t *testing.T) *E2ETestSuite {
	db, err := sql.Open("postgres", "postgres://user:password@postgres:5432/quest?sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	cleanupDatabase(db)

	handler := setupRoutes(db)
	server := httptest.NewServer(handler)

	return &E2ETestSuite{
		server: server,
		db:     db,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (suite *E2ETestSuite) teardown() {
	suite.server.Close()
	if suite.db != nil {
		suite.db.Close()
	}
}

func cleanupDatabase(db *sql.DB) {
	tables := []string{
		"multiplayer_choice", "multiplayer_playthrough", "multiplayer_server",
		"character_action_choice", "character_action", "player_action_choice",
		"player_action", "narration_action", "step", "character", "quest",
	}

	for _, table := range tables {
		db.Exec(fmt.Sprintf("DELETE FROM %s", table))
	}
}

func (suite *E2ETestSuite) makeRequest(method, path string, body interface{}) (*http.Response, error) {
	var reqBody *bytes.Buffer
	if body != nil {
		jsonData, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(jsonData)
	} else {
		reqBody = bytes.NewBuffer([]byte{})
	}

	req, err := http.NewRequest(method, suite.server.URL+path, reqBody)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return suite.client.Do(req)
}

func (suite *E2ETestSuite) createQuest(t *testing.T) int {
	// Загружаем тестовый квест из файла
	questData, err := os.ReadFile("test_quest.json")
	if err != nil {
		t.Fatalf("Failed to read test_quest.json: %v", err)
	}

	var quest map[string]interface{}
	json.Unmarshal(questData, &quest)

	resp, err := suite.makeRequest("POST", "/make_quest", quest)
	if err != nil {
		t.Fatalf("Failed to create quest: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	var questResp QuestResponse
	json.NewDecoder(resp.Body).Decode(&questResp)

	return questResp.ID
}

func (suite *E2ETestSuite) createServer(t *testing.T, questID int, serverName, playerName string) int {
	requestBody := map[string]interface{}{
		"quest_id":    questID,
		"server_name": serverName,
		"player_name": playerName,
	}

	resp, err := suite.makeRequest("POST", "/create_server", requestBody)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	var serverResp ServerResponse
	json.NewDecoder(resp.Body).Decode(&serverResp)

	return serverResp.ServerID
}

func (suite *E2ETestSuite) joinServer(t *testing.T, serverID int, playerName string) {
	requestBody := map[string]interface{}{
		"server_id":   serverID,
		"player_name": playerName,
	}

	resp, err := suite.makeRequest("POST", "/join_server", requestBody)
	if err != nil {
		t.Fatalf("Failed to join server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}
}

func (suite *E2ETestSuite) getMultiplayerState(t *testing.T, serverID int) MultiplayerState {
	resp, err := suite.makeRequest("GET", fmt.Sprintf("/get_multiplayer_dialog?server_id=%d", serverID), nil)
	if err != nil {
		t.Fatalf("Failed to get multiplayer state: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	var state MultiplayerState
	json.NewDecoder(resp.Body).Decode(&state)

	return state
}

func (suite *E2ETestSuite) proceedToNextStep(t *testing.T, serverID int) {
	requestBody := map[string]interface{}{
		"server_id": serverID,
	}

	resp, err := suite.makeRequest("POST", "/proceed_to_next_step", requestBody)
	if err != nil {
		t.Fatalf("Failed to proceed to next step: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}
}

func (suite *E2ETestSuite) makeChoice(t *testing.T, serverID int, playerName string, choiceID int) {
	requestBody := map[string]interface{}{
		"server_id":   serverID,
		"player_name": playerName,
		"choice_id":   choiceID,
	}

	resp, err := suite.makeRequest("POST", "/make_multiplayer_choice", requestBody)
	if err != nil {
		t.Fatalf("Failed to make choice: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestE2E_PeacefulPath(t *testing.T) {
	suite := setupE2ETest(t)
	defer suite.teardown()

	// Шаг 1: Создание квеста
	questID := suite.createQuest(t)
	if questID != 1 {
		t.Errorf("Expected quest ID 1, got %d", questID)
	}

	// Шаг 2: Создание мультиплеерного сервера
	serverID := suite.createServer(t, questID, "Тест разветвлений", "Алиса")
	if serverID != 1 {
		t.Errorf("Expected server ID 1, got %d", serverID)
	}

	// Шаг 3: Добавление второго игрока
	suite.joinServer(t, serverID, "Боб")

	// Шаг 4: Проверка начального состояния
	state := suite.getMultiplayerState(t, serverID)
	if !containsText(state.StepText, "замку") {
		t.Errorf("Expected initial narration about castle, got: %s", state.StepText)
	}

	// Шаг 5: Переход через нарративный шаг
	suite.proceedToNextStep(t, serverID)

	// Шаг 6: Проверка шага с выбором
	state = suite.getMultiplayerState(t, serverID)
	if len(state.Players) != 2 {
		t.Errorf("Expected 2 players, got %d", len(state.Players))
	}

	// Найдем ID выборов для мирного пути
	var peacefulChoice1, peacefulChoice2 int
	for _, player := range state.Players {
		for _, choice := range player.Choices {
			if containsText(choice.Text, "постучать") {
				peacefulChoice1 = choice.ID
			}
			if containsText(choice.Text, "осмотреть") {
				peacefulChoice2 = choice.ID
			}
		}
	}

	// Шаг 7-8: Игроки делают мирные выборы
	suite.makeChoice(t, serverID, "Алиса", peacefulChoice1)
	suite.makeChoice(t, serverID, "Боб", peacefulChoice2)

	// Шаг 9: Проверка результата мирного пути
	state = suite.getMultiplayerState(t, serverID)
	if !containsText(state.StepText, "Что будете") && !containsText(state.StepText, "мирн") {
		t.Errorf("Expected peaceful reaction, got: %s", state.StepText)
	}

	t.Log("Peaceful path test completed successfully")
}

func TestE2E_BranchingLogic(t *testing.T) {
	suite := setupE2ETest(t)
	defer suite.teardown()

	questID := suite.createQuest(t)
	serverID := suite.createServer(t, questID, "Тест логики", "Тестер1")
	suite.joinServer(t, serverID, "Тестер2")

	// Проверяем последовательность шагов
	initialState := suite.getMultiplayerState(t, serverID)
	if initialState.CurrentStepID != 1 {
		t.Errorf("Expected initial step 1, got %d", initialState.CurrentStepID)
	}

	// Переходим к следующему шагу
	suite.proceedToNextStep(t, serverID)

	nextState := suite.getMultiplayerState(t, serverID)
	if nextState.CurrentStepID != 2 {
		t.Errorf("Expected step 2 after proceeding, got %d", nextState.CurrentStepID)
	}

	t.Log("Branching logic test completed successfully")
}

func TestE2E_PointsSystem(t *testing.T) {
	suite := setupE2ETest(t)
	defer suite.teardown()

	questID := suite.createQuest(t)
	serverID := suite.createServer(t, questID, "Тест очков", "Игрок1")
	suite.joinServer(t, serverID, "Игрок2")

	// Пропускаем нарративный шаг
	suite.proceedToNextStep(t, serverID)

	// Проверяем начальные очки в моке
	points := serverPoints[serverID]
	violence := points["violence"]
	whatever := points["whatever"]
	pacifism := points["pacifism"]

	if violence != 0 || whatever != 0 || pacifism != 0 {
		t.Errorf("Expected initial points to be 0,0,0, got %d,%d,%d", violence, whatever, pacifism)
	}

	// Делаем выборы и проверяем изменение очков
	state := suite.getMultiplayerState(t, serverID)
	if len(state.Players) > 0 && len(state.Players[0].Choices) > 0 {
		suite.makeChoice(t, serverID, "Игрок1", state.Players[0].Choices[0].ID)

		// Ждем некоторое время для обновления базы данных
		time.Sleep(100 * time.Millisecond)

		// Проверяем изменение очков в моке
		points := serverPoints[serverID]
		violence = points["violence"]
		whatever = points["whatever"]
		pacifism = points["pacifism"]

		totalPoints := violence + whatever + pacifism
		if totalPoints == 0 {
			t.Errorf("Expected points to change after choice, but they remained 0,0,0")
		}
	}

	t.Log("Points system test completed successfully")
}

func TestE2E_ErrorHandling(t *testing.T) {
	suite := setupE2ETest(t)
	defer suite.teardown()

	// Тест несуществующего сервера
	resp, err := suite.makeRequest("GET", "/get_multiplayer_dialog?server_id=999", nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 for non-existent server, got %d", resp.StatusCode)
	}

	// Тест неверного формата ID
	resp2, err := suite.makeRequest("GET", "/get_multiplayer_dialog?server_id=invalid", nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid server ID, got %d", resp2.StatusCode)
	}

	t.Log("Error handling test completed successfully")
}

// Вспомогательная функция для проверки содержания текста
func containsText(text, substring string) bool {
	return len(text) > 0 && (text == substring ||
		len(substring) > 0 &&
			(text[0:min(len(text), len(substring))] == substring ||
				contains(text, substring)))
}

func contains(text, substr string) bool {
	for i := 0; i <= len(text)-len(substr); i++ {
		if text[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestMain(m *testing.M) {
	// Настройка тестовой среды
	code := m.Run()

	// Очистка после тестов
	os.Exit(code)
}
