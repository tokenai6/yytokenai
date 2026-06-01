package shared

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

func ResolveSessionDayWindow(ctx context.Context, bizDate time.Time) (time.Time, time.Time, error) {
	row, err := g.DB().GetOne(ctx, `
		SELECT MIN(start_time) AS day_start, MAX(end_time) AS day_end
		FROM group_match_session
		WHERE session_date = ?::date
	`, bizDate.Format("2006-01-02"))
	if err != nil {
		return time.Time{}, time.Time{}, gerror.Wrap(err, "query group match session day window failed")
	}
	if row == nil {
		return time.Time{}, time.Time{}, nil
	}
	return row["day_start"].Time(), row["day_end"].Time(), nil
}
