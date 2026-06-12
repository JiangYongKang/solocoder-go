package eventsrc

import (
	"encoding/json"
	"errors"
)

type TestAccount struct {
	BaseAggregate
	Owner   string
	Balance float64
	Active  bool
}

func NewTestAccount(id string) *TestAccount {
	return &TestAccount{
		BaseAggregate: *NewBaseAggregate(id),
	}
}

func (a *TestAccount) Apply(event *Event) error {
	switch event.EventType {
	case "AccountCreated":
		var data struct {
			Owner string `json:"owner"`
		}
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		a.Owner = data.Owner
		a.Active = true
	case "Deposit":
		var data struct {
			Amount float64 `json:"amount"`
		}
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		a.Balance += data.Amount
	case "Withdraw":
		var data struct {
			Amount float64 `json:"amount"`
		}
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		if a.Balance < data.Amount {
			return errors.New("insufficient balance")
		}
		a.Balance -= data.Amount
	case "AccountDeactivated":
		a.Active = false
	default:
		return errors.New("unknown event type: " + event.EventType)
	}

	a.IncrementVersion()
	return nil
}

func (a *TestAccount) MarshalState() ([]byte, error) {
	state := struct {
		Owner   string  `json:"owner"`
		Balance float64 `json:"balance"`
		Active  bool    `json:"active"`
	}{
		Owner:   a.Owner,
		Balance: a.Balance,
		Active:  a.Active,
	}
	return json.Marshal(state)
}

func (a *TestAccount) UnmarshalState(data []byte) error {
	var state struct {
		Owner   string  `json:"owner"`
		Balance float64 `json:"balance"`
		Active  bool    `json:"active"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	a.Owner = state.Owner
	a.Balance = state.Balance
	a.Active = state.Active
	return nil
}
