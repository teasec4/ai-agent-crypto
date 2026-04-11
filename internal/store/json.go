package store

type Profile struct{
	Name string `json:"name"`
	Balance int `json:"balance"`
}

func NewProfile(name string, balance int)*Profile{
	return &Profile{
		Name: name,
		Balance: balance,
	}
}


