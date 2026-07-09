package app_test

import (
	"testing"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/agents/app"
)

func strPtr(v string) *string { return &v }

func TestCloseDateHygiene_ComputeCloseDateFlags(t *testing.T) {
	today := time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)

	t.Run("nil expected close date is missing only", func(t *testing.T) {
		got := app.ComputeCloseDateFlags("open", nil, today, 10, false)
		if !got.Missing || got.Overdue || got.UnrealisticSoon || got.UnrealisticStale {
			t.Fatalf("flags = %+v, want Missing only", got)
		}
	})

	t.Run("exactly today is not overdue", func(t *testing.T) {
		date := today
		got := app.ComputeCloseDateFlags("open", &date, today, 39, false)
		if got.Overdue {
			t.Fatalf("flags = %+v, want not overdue", got)
		}
		if !got.UnrealisticSoon {
			t.Fatalf("flags = %+v, want unrealistic soon", got)
		}
	})

	t.Run("today plus seven is unrealistic soon", func(t *testing.T) {
		date := today.AddDate(0, 0, 7)
		got := app.ComputeCloseDateFlags("open", &date, today, 39, false)
		if !got.UnrealisticSoon {
			t.Fatalf("flags = %+v, want unrealistic soon", got)
		}
	})

	t.Run("today plus eight is not unrealistic soon", func(t *testing.T) {
		date := today.AddDate(0, 0, 8)
		got := app.ComputeCloseDateFlags("open", &date, today, 39, false)
		if got.UnrealisticSoon {
			t.Fatalf("flags = %+v, want not unrealistic soon", got)
		}
	})

	t.Run("already overdue never trips unrealistic soon", func(t *testing.T) {
		date := today.AddDate(0, 0, -1)
		got := app.ComputeCloseDateFlags("open", &date, today, 10, false)
		if !got.Overdue {
			t.Fatalf("flags = %+v, want overdue", got)
		}
		if got.UnrealisticSoon {
			t.Fatalf("flags = %+v, want not unrealistic soon", got)
		}
	})

	t.Run("win probability at 40 never trips unrealistic soon", func(t *testing.T) {
		date := today.AddDate(0, 0, 1)
		got := app.ComputeCloseDateFlags("open", &date, today, 40, false)
		if got.UnrealisticSoon {
			t.Fatalf("flags = %+v, want not unrealistic soon", got)
		}
	})

	t.Run("stale only needs stalled and date within sixty days", func(t *testing.T) {
		date := today.AddDate(0, 0, 60)
		got := app.ComputeCloseDateFlags("open", &date, today, 10, true)
		if !got.UnrealisticStale {
			t.Fatalf("flags = %+v, want unrealistic stale", got)
		}
	})

	t.Run("won deals never flag", func(t *testing.T) {
		date := today.AddDate(0, 0, -100)
		got := app.ComputeCloseDateFlags("won", &date, today, 10, true)
		if got.Any() {
			t.Fatalf("flags = %+v, want none", got)
		}
	})
}

func TestCloseDateHygiene_StageVelocityDays(t *testing.T) {
	if got := app.StageVelocityDays(23, 19); got != app.CloseDateStageDays {
		t.Fatalf("StageVelocityDays(19) = %d, want %d", got, app.CloseDateStageDays)
	}
	if got := app.StageVelocityDays(23, 20); got != 23 {
		t.Fatalf("StageVelocityDays(20) = %d, want 23", got)
	}
}

func TestCloseDateHygiene_ProposedCloseDate(t *testing.T) {
	today := time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)
	got := app.ProposedCloseDate(today, 0, 18)
	if !got.Equal(today.AddDate(0, 0, 18)) {
		t.Fatalf("ProposedCloseDate(0,18) = %s, want %s", got, today.AddDate(0, 0, 18))
	}
	got = app.ProposedCloseDate(today, 3, 18)
	if !got.Equal(today.AddDate(0, 0, 54)) {
		t.Fatalf("ProposedCloseDate(3,18) = %s, want %s", got, today.AddDate(0, 0, 54))
	}
}

func TestCloseDateHygiene_InForecastCommit(t *testing.T) {
	commit := "commit"
	pipeline := "pipeline"

	if app.InForecastCommit(nil, 89) {
		t.Fatal("nil category, 89 should not be commit")
	}
	if !app.InForecastCommit(nil, 90) {
		t.Fatal("nil category, 90 should be commit")
	}
	if !app.InForecastCommit(strPtr(commit), 10) {
		t.Fatal("commit category should win regardless of probability")
	}
	if app.InForecastCommit(strPtr(pipeline), 95) {
		t.Fatal("pipeline category should not be commit")
	}
}

func TestCloseDateHygiene_DecideCloseDateAction(t *testing.T) {
	if got := app.DecideCloseDateAction(true, true, true, false, false, true); got != app.ActionDowngradeAndReview {
		t.Fatalf("quiet action = %s, want %s", got, app.ActionDowngradeAndReview)
	}
	if got := app.DecideCloseDateAction(false, true, true, false, false, false); got != app.ActionProvisionalConfirm {
		t.Fatalf("autoApply disabled = %s, want %s", got, app.ActionProvisionalConfirm)
	}
	if got := app.DecideCloseDateAction(false, true, true, false, true, true); got != app.ActionProvisionalConfirm {
		t.Fatalf("late stage = %s, want %s", got, app.ActionProvisionalConfirm)
	}
	if got := app.DecideCloseDateAction(false, true, true, true, false, true); got != app.ActionProvisionalConfirm {
		t.Fatalf("in forecast commit = %s, want %s", got, app.ActionProvisionalConfirm)
	}
	if got := app.DecideCloseDateAction(false, true, true, false, false, true); got != app.ActionAutoApply {
		t.Fatalf("eligible action = %s, want %s", got, app.ActionAutoApply)
	}
}
