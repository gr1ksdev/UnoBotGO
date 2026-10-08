package ranking

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/malbs/UnoGoBot/internal/groups"
)

// GlobalRow is a storage projection, never a public HTTP DTO.
type GlobalRow struct {
	ID        int64
	Name      string
	Score     Units
	Activity  time.Time
	Placement int
	Position  int64
	Anonymous bool
}

type PageKey struct {
	Score     Units     `json:"score"`
	Activity  time.Time `json:"activity"`
	Name      string    `json:"name"`
	ID        int64     `json:"id"`
	Placement int       `json:"placement"`
}

type GlobalRequest struct {
	LookupID int64
	Kind     string
	System   groups.RankingSystem
	GroupID  int64
	Month    time.Time
	Limit    int
	After    *PageKey
}

type GlobalPage struct {
	Rows  []GlobalRow
	Group *GlobalRow
	More  bool
	Month time.Time
}

type GlobalRepository interface {
	ReadGlobalRanking(context.Context, GlobalRequest) (GlobalPage, error)
}

type GlobalService struct {
	Repository GlobalRepository
	Now        func() time.Time
}

var ErrPeriodChanged = errors.New("ranking period changed")
var ErrNotFound = errors.New("ranking not found")

func (s *GlobalService) CurrentMonth() time.Time {
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	return MonthStart(now)
}

func (s *GlobalService) List(ctx context.Context, req GlobalRequest) (GlobalPage, error) {
	month := s.CurrentMonth()
	if !req.Month.IsZero() && !req.Month.Equal(month) {
		return GlobalPage{}, ErrPeriodChanged
	}
	req.Month = month
	if req.System != groups.Updated && req.System != groups.Legacy {
		return GlobalPage{}, ErrInvalid
	}
	if req.Kind != "groups" && req.Kind != "players" && req.Kind != "detail" {
		return GlobalPage{}, ErrInvalid
	}
	if req.Kind == "detail" && req.GroupID == 0 {
		return GlobalPage{}, ErrInvalid
	}
	if req.Limit == 0 {
		req.Limit = 50
	}
	if req.Limit < 1 || req.Limit > 100 {
		return GlobalPage{}, ErrInvalid
	}
	page, err := s.Repository.ReadGlobalRanking(ctx, req)
	page.Month = month
	return page, err
}

func MaskID(id int64) string {
	digits := strings.TrimPrefix(strconv.FormatInt(id, 10), "-")
	if len(digits) > 4 {
		digits = digits[len(digits)-4:]
	}
	return "••••" + digits
}

func (r GlobalRow) Key() PageKey { return PageKey{r.Score, r.Activity, r.Name, r.ID, r.Placement} }
