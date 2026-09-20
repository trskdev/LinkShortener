package domain

type Link struct {
	ID        int64  `json:"id"`
	ShortCode string `json:"short_code"`
	LongUrl   string `json:"long_url"`
	Clicks    int32  `json:"clicks"`
}

func (l *Link) Validate() error {
	if l.LongUrl == "" {
		return ErrNoURLProvided
	}

	if l.ShortCode == "" {
		return ErrNoCodeProvided
	}

	return nil
}
