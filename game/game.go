package game

type TokenData struct {
	Name      string  `json:"name"`
	ImgPath   string  `json:"imgPath"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	TokenSize float64 `json:"tokenSize"`
}

type State struct {
	DisplayedTokens   map[string]TokenData `json:"displayedTokens"`
	BackgroundImgPath string               `json:"backgroundImgPath"`
	ShowGrid          bool                 `json:"showGrid"`
	GridUnit          float64              `json:"gridUnit"`
}

func NewState() State {
	return State{
		DisplayedTokens:   make(map[string]TokenData),
		BackgroundImgPath: "/assets/default/maps/tavern.jpg",
		ShowGrid:          true,
		GridUnit:          96,
	}
}

func (s *State) AddToken(id string, token TokenData) {
	s.DisplayedTokens[id] = token
}

func (s *State) MoveToken(id string, x, y float64) {
	if token, ok := s.DisplayedTokens[id]; ok {
		token.X = x
		token.Y = y
		s.DisplayedTokens[id] = token
	}
}

func (s *State) DeleteToken(id string) {
	delete(s.DisplayedTokens, id)
}

func (s *State) ClearTokens() {
	s.DisplayedTokens = make(map[string]TokenData)
}

func (s *State) ChangeBackgroundImg(path string) {
	s.BackgroundImgPath = path
}

func (s *State) ToggleGrid() {
	s.ShowGrid = !s.ShowGrid
}

// Paloads marshalling
type AddTokenPayload struct {
	ID    string    `json:"id"`
	Token TokenData `json:"token"`
}

type MoveTokenPayload struct {
	ID string  `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
}

type DeleteTokenPayload struct {
	ID string `json:"id"`
}

type ChangeBackgroundPayload struct {
	ImgPath string `json:"imgPath"`
}
