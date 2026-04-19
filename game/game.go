package game

type TokenData struct {
	Name      string  `json:"name"`
	ImgPath   string  `json:"imgPath"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	TokenSize float64 `json:"tokenSize"`
}

type AreaTemplate struct {
	Shape   string  `json:"shape"`   // "circle" or "square"
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Size    float64 `json:"size"`    // in grid units
	Color   string  `json:"color"`   // hex e.g. "#ff0000"
	Opacity float64 `json:"opacity"` // 0.0–1.0
}

type State struct {
	DisplayedTokens   map[string]TokenData    `json:"displayedTokens"`
	BackgroundImgPath string                  `json:"backgroundImgPath"`
	ShowGrid          bool                    `json:"showGrid"`
	GridUnit          float64                 `json:"gridUnit"`
	AreaTemplates     map[string]AreaTemplate `json:"areaTemplates"`
}

func NewState() State {
	return State{
		DisplayedTokens:   make(map[string]TokenData),
		BackgroundImgPath: "/assets/default/maps/tavern.jpg",
		ShowGrid:          true,
		GridUnit:          96,
		AreaTemplates:     make(map[string]AreaTemplate),
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

func (s *State) AddAreaTemplate(id string, t AreaTemplate) {
	s.AreaTemplates[id] = t
}

func (s *State) MoveAreaTemplate(id string, x, y float64) {
	if t, ok := s.AreaTemplates[id]; ok {
		t.X = x
		t.Y = y
		s.AreaTemplates[id] = t
	}
}

func (s *State) DeleteAreaTemplate(id string) {
	delete(s.AreaTemplates, id)
}

func (s *State) ClearAreaTemplates() {
	s.AreaTemplates = make(map[string]AreaTemplate)
}

// Payload types for marshalling
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

type AddAreaTemplatePayload struct {
	ID       string       `json:"id"`
	Template AreaTemplate `json:"template"`
}

type MoveAreaTemplatePayload struct {
	ID string  `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
}

type DeleteAreaTemplatePayload struct {
	ID string `json:"id"`
}
