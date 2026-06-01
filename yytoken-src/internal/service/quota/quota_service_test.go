package quota

import (
	"context"
	"errors"
	"testing"

	"github.com/shopspring/decimal"
)

type fakeRewardStatRepo struct {
	direct      decimal.Decimal
	indirect    decimal.Decimal
	directErr   error
	indirectErr error
}

func (f *fakeRewardStatRepo) GetDirectRewardSum(ctx context.Context, userID int64) (decimal.Decimal, error) {
	if f.directErr != nil {
		return decimal.Zero, f.directErr
	}
	return f.direct, nil
}

func (f *fakeRewardStatRepo) GetIndirectRewardSum(ctx context.Context, userID int64) (decimal.Decimal, error) {
	if f.indirectErr != nil {
		return decimal.Zero, f.indirectErr
	}
	return f.indirect, nil
}

func TestGetReceivedReward_SumsDirectAndIndirect(t *testing.T) {
	svc := &quotaService{
		rewardStatRepo: &fakeRewardStatRepo{
			direct:   decimal.NewFromInt(100),
			indirect: decimal.NewFromInt(50),
		},
	}

	got, err := svc.GetReceivedReward(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetReceivedReward returned error: %v", err)
	}

	want := decimal.NewFromInt(150)
	if !got.Equal(want) {
		t.Fatalf("unexpected total reward: got=%s want=%s", got.String(), want.String())
	}
}

func TestGetReceivedReward_ReturnsDirectError(t *testing.T) {
	svc := &quotaService{
		rewardStatRepo: &fakeRewardStatRepo{
			directErr: errors.New("direct query failed"),
		},
	}

	_, err := svc.GetReceivedReward(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetReceivedReward_ReturnsIndirectError(t *testing.T) {
	svc := &quotaService{
		rewardStatRepo: &fakeRewardStatRepo{
			direct:      decimal.NewFromInt(100),
			indirectErr: errors.New("indirect query failed"),
		},
	}

	_, err := svc.GetReceivedReward(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
