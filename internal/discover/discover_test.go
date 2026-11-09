package discover

import (
	"go/types"
	"slices"
	"strings"
	"testing"

	"github.com/leighstillard/timetell/internal/load"
)

func names[T any](xs []T, name func(T) string) string {
	var out []string
	for _, x := range xs {
		out = append(out, name(x))
	}
	return strings.Join(out, ",")
}

func TestDiscover(t *testing.T) {
	cases := []struct {
		dir, roots, acts, signals string
		fallback                  bool
	}{
		{dir: "money_transfer", roots: "MoneyTransfer", acts: "CurrencyConvert,DepositFunds,NotifyATO,TakeNonResidentTax,WithdrawFunds"},
		{dir: "parent_child", roots: "OrderWorkflow,PaymentWorkflow", acts: "ManagerApproval,ProcessPayment,SendConfirmation,ValidateOrder,Verify3DS"},
		{dir: "signal_flow", roots: "NotificationWorkflow,OrderWorkflow,ShippingWorkflow", acts: "CompleteOrder,ProcessOrder,SendNotification,ShipPackage",
			signals: "NotificationWorkflow=notify_shipped;OrderWorkflow=;ShippingWorkflow=ship_order"},
		{dir: "classify", roots: "Classify", acts: "Charge", signals: "Classify=approve stop"},
		{dir: "unregistered_activity", roots: "Unregistered", acts: "Charge"},
		{dir: "fallback", roots: "Fallback", fallback: true},
	}
	for _, c := range cases {
		t.Run(c.dir, func(t *testing.T) {
			pkgs, err := load.Load("../../testdata", "./"+c.dir)
			if err != nil {
				t.Fatal(err)
			}
			ix := Discover(pkgs)
			if got := names(ix.Roots, func(f *Func) string { return f.Obj.Name() }); got != c.roots {
				t.Errorf("roots %q, want %q", got, c.roots)
			}
			if got := names(ix.Activities, (*types.Func).Name); got != c.acts {
				t.Errorf("activities %q, want %q", got, c.acts)
			}
			if ix.Fallback != c.fallback || ix.Registered == c.fallback {
				t.Errorf("fallback %v registered %v, want fallback %v", ix.Fallback, ix.Registered, c.fallback)
			}
			if c.signals != "" {
				var got []string
				for _, r := range ix.Roots {
					got = append(got, r.Obj.Name()+"="+strings.Join(ix.Signals[r.Obj], " "))
				}
				slices.Sort(got)
				if s := strings.Join(got, ";"); s != c.signals {
					t.Errorf("signals %q, want %q", s, c.signals)
				}
			}
		})
	}
}
