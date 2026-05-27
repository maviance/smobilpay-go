// Package main — scenarios.go is the scenario harness for the Smobilpay
// Go smoke-test runner. It is structured as one method per scenario on a
// shared Harness, plus a handful of shared helpers (quote-and-report,
// collect-and-report, TRID generation, amount resolution).
//
// Each collection-style scenario discovers a catalog item and quotes by
// default. When the matching config block sets `collect: true` (plus
// customerPhonenumber + customerEmailaddress), the harness also calls
// /v2/collectstd, then polls /v2/verifytx after a short settle. This
// matches the nodejs sample's collect opt-in pattern so the same
// smoke-test.json file drives the Java, Node, and Go runners.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"time"

	smob "github.com/maviance/smobilpay-go"
)

const sep = "----------------------------------------------------------------------"

// Harness owns counters and the smobilpay client for one smoketest run.
type Harness struct {
	client *smob.Client
	cfg    SmokeConfig

	passed  int
	failed  int
	skipped int
}

// collectOpts is the flow-agnostic projection of the collect opt-in
// fields that each block carries. Each scenarioXxx builds one of these
// from its block before calling quoteAndReport, which keeps the
// quote/collect helpers free of any block-specific knowledge.
type collectOpts struct {
	Collect              bool
	CustomerPhoneNumber  string
	CustomerEmailAddress string
	ServiceNumber        string
	CustomerName         string
	CustomerAddress      string
	CustomerNumber       string
	TRID                 string
	Tag                  string
	CallbackURL          string
	CData                string
}

// skipError is used by scenarios to signal "this is an expected skip" —
// caught by run() and recorded as SKIP rather than FAIL.
type skipError struct{ msg string }

func (e skipError) Error() string { return e.msg }

func skip(format string, args ...any) error {
	return skipError{msg: fmt.Sprintf(format, args...)}
}

// run wraps a scenario function with the boilerplate banner +
// pass/skip/fail bookkeeping.
func (h *Harness) run(name string, fn func() error) {
	fmt.Println(sep)
	fmt.Println("RUN  " + name)
	err := fn()
	switch {
	case err == nil:
		h.passed++
		fmt.Println("PASS " + name)
	case isSkip(err):
		var se skipError
		errors.As(err, &se)
		h.skipped++
		fmt.Println("SKIP " + name + " - " + se.msg)
	default:
		h.failed++
		fmt.Println("FAIL " + name + " - " + classify(err))
		printAPIErrorDetail(err)
	}
}

func isSkip(err error) bool {
	var se skipError
	return errors.As(err, &se)
}

func classify(err error) string {
	var ae *smob.AuthError
	if errors.As(err, &ae) {
		return fmt.Sprintf("auth error (HTTP %d, oauth %s): %s",
			ae.HTTPStatus, ae.OAuthError, ae.Message)
	}
	var pe *smob.APIError
	if errors.As(err, &pe) {
		return fmt.Sprintf("API error (HTTP %d)", pe.HTTPStatus)
	}
	var te *smob.TransportError
	if errors.As(err, &te) {
		return fmt.Sprintf("transport error on %s: %v", te.Op, te.Cause)
	}
	return err.Error()
}

func printAPIErrorDetail(err error) {
	var pe *smob.APIError
	if !errors.As(err, &pe) {
		return
	}
	detail(fmt.Sprintf("respCode: %d", pe.RespCode))
	detail(fmt.Sprintf("devMsg:   %s", pe.DevMsg))
	if pe.UsrMsg != "" {
		detail(fmt.Sprintf("usrMsg:   %s", pe.UsrMsg))
	}
	if pe.Link != "" {
		detail(fmt.Sprintf("link:     %s", pe.Link))
	}
}

// detail prints a single indented detail line under a RUN/PASS banner.
func detail(line string) { fmt.Println("     " + line) }

// resolveAmount picks the amount to send to /v2/quotestd: caller-
// supplied wins; otherwise we fall back to the catalog amount if the
// item has a fixed price.
func resolveAmount(item smob.PaymentItem, cfgAmount int) (int, error) {
	if cfgAmount > 0 {
		return cfgAmount, nil
	}
	if a := item.AmountLocalCur(); a != nil && *a >= 1 {
		return int(*a), nil
	}
	return 0, fmt.Errorf("item %s has no fixed catalog amount; set \"amount\" in this block",
		item.PayItemID())
}

// generateTRID returns a caller-side TRID of the form
// "go-smoke-<unix-millis>-<6 hex>". Sufficient for human-readable
// log correlation; not a security-sensitive value.
func generateTRID() string {
	buf := make([]byte, 3)
	if _, err := rand.Read(buf); err != nil {
		// Fall back to a timestamp-only TRID rather than failing the run.
		return fmt.Sprintf("go-smoke-%d-000000", time.Now().UnixMilli())
	}
	return fmt.Sprintf("go-smoke-%d-%s", time.Now().UnixMilli(), hex.EncodeToString(buf))
}

// quoteAndReport quotes the given item and prints the quote, then —
// depending on opts.Collect — either calls collectAndReport or prints a
// one-liner explaining how to opt in.
func quoteAndReport(ctx context.Context, client *smob.Client, item smob.PaymentItem, amount int, opts collectOpts) error {
	q, err := client.Initiate.Quote(ctx, smob.QuoteRequest{Amount: amount, PayItemID: item.PayItemID()})
	if err != nil {
		return err
	}
	detail("quoteId:        " + q.QuoteID)
	detail("expiresAt:      " + q.ExpiresAt.Format(time.RFC3339))
	var pl, ps float64
	if q.PriceLocalCur != nil {
		pl = *q.PriceLocalCur
	}
	if q.PriceSystemCur != nil {
		ps = *q.PriceSystemCur
	}
	detail(fmt.Sprintf("price (local):  %g %s", pl, q.LocalCur))
	detail(fmt.Sprintf("price (system): %g %s", ps, q.SystemCur))
	detail("promotion:      " + q.Promotion)

	if opts.Collect {
		return collectAndReport(ctx, client, q, opts)
	}
	detail(`(intentionally NOT calling /v2/collectstd — set "collect": true on this block to enable)`)
	return nil
}

// collectAndReport executes /v2/collectstd against an unexpired quote.
// WARNING: this moves real money on the partner balance.
func collectAndReport(ctx context.Context, client *smob.Client, quote smob.QuoteResponse, opts collectOpts) error {
	if opts.CustomerPhoneNumber == "" {
		return errors.New("'collect: true' requires 'customerPhonenumber' on the same block")
	}
	if opts.CustomerEmailAddress == "" {
		return errors.New("'collect: true' requires 'customerEmailaddress' on the same block")
	}

	req := smob.CollectionRequest{
		QuoteID:              quote.QuoteID,
		CustomerPhoneNumber:  opts.CustomerPhoneNumber,
		CustomerEmailAddress: opts.CustomerEmailAddress,
	}
	if opts.ServiceNumber != "" {
		req.ServiceNumber = opts.ServiceNumber
	}
	if opts.CustomerName != "" {
		req.CustomerName = opts.CustomerName
	}
	if opts.CustomerAddress != "" {
		req.CustomerAddress = opts.CustomerAddress
	}
	if opts.CustomerNumber != "" {
		req.CustomerNumber = opts.CustomerNumber
	}
	if opts.Tag != "" {
		req.Tag = opts.Tag
	}
	if opts.CallbackURL != "" {
		req.CallbackURL = opts.CallbackURL
	}
	if opts.CData != "" {
		req.CData = opts.CData
	}
	if opts.TRID != "" {
		req.TRID = opts.TRID
	} else {
		req.TRID = generateTRID()
	}

	detail("-> POST /v2/collectstd")
	detail("   trid:    " + req.TRID)

	resp, err := client.Confirm.Collect(ctx, req)
	if err != nil {
		return err
	}

	detail("ptn:             " + resp.PTN)
	detail(fmt.Sprintf("status:          %s", resp.Status))
	detail("receiptNumber:   " + resp.ReceiptNumber)
	if resp.VeriCode != "" {
		detail("veriCode:        " + resp.VeriCode)
	}
	var bal, pl, ps float64
	if resp.AgentBalance != nil {
		bal = *resp.AgentBalance
	}
	if resp.PriceLocalCur != nil {
		pl = *resp.PriceLocalCur
	}
	if resp.PriceSystemCur != nil {
		ps = *resp.PriceSystemCur
	}
	detail(fmt.Sprintf("agentBalance:    %g", bal))
	detail(fmt.Sprintf("price (local):   %g %s", pl, resp.LocalCur))
	detail(fmt.Sprintf("price (system):  %g %s", ps, resp.SystemCur))
	detail("timestamp:       " + resp.Timestamp.Format(time.RFC3339))
	if resp.PIN != "" {
		detail("pin:             " + resp.PIN)
	}

	// Brief settle so the server has a moment to advance the row before
	// we re-check via /v2/verifytx.
	time.Sleep(2 * time.Second)

	verifications, err := client.Verify.VerifyTransaction(ctx, resp.PTN, "")
	if err != nil {
		// The collect already succeeded; don't fail the scenario over a
		// hiccup polling the verify endpoint.
		detail("verifytx:        warning: " + err.Error())
		return nil
	}
	if len(verifications) > 0 {
		v := verifications[0]
		detail(fmt.Sprintf("verifytx:        status=%s clearingDate=%s",
			v.Status, v.ClearingDate.Format("2006-01-02")))
	} else {
		detail("verifytx:        no rows yet (final status will land via callbackUrl or a later poll)")
	}
	return nil
}

// --- collectOpts adapters per block type ------------------------------
//
// Each block defines its collect opt-in surface differently in config.go
// (most embed CollectOpts; BillCfg/SubscriptionCfg spell the fields out
// because they reserve ServiceNumber/CustomerNumber as discovery
// parameters). These adapters project each block into the flow-agnostic
// collectOpts struct used by quoteAndReport.

func cashoutCollectOpts(c *CashoutCfg) collectOpts {
	return collectOpts{
		Collect:              c.Collect,
		CustomerPhoneNumber:  c.CustomerPhoneNumber,
		CustomerEmailAddress: c.CustomerEmailAddress,
		ServiceNumber:        c.ServiceNumber,
		CustomerName:         c.CustomerName,
		CustomerAddress:      c.CustomerAddress,
		CustomerNumber:       c.CustomerNumber,
		TRID:                 c.TRID,
		Tag:                  c.Tag,
		CallbackURL:          c.CallbackURL,
		CData:                c.CData,
	}
}

func billCollectOpts(c *BillCfg) collectOpts {
	return collectOpts{
		Collect:              c.Collect,
		CustomerPhoneNumber:  c.CustomerPhoneNumber,
		CustomerEmailAddress: c.CustomerEmailAddress,
		ServiceNumber:        c.ServiceNumber,
		CustomerName:         c.CustomerName,
		CustomerAddress:      c.CustomerAddress,
		CustomerNumber:       c.CustomerNumber,
		TRID:                 c.TRID,
		Tag:                  c.Tag,
		CallbackURL:          c.CallbackURL,
		CData:                c.CData,
	}
}

func topupCollectOpts(c *TopupCfg) collectOpts {
	return collectOpts{
		Collect:              c.Collect,
		CustomerPhoneNumber:  c.CustomerPhoneNumber,
		CustomerEmailAddress: c.CustomerEmailAddress,
		ServiceNumber:        c.ServiceNumber,
		CustomerName:         c.CustomerName,
		CustomerAddress:      c.CustomerAddress,
		CustomerNumber:       c.CustomerNumber,
		TRID:                 c.TRID,
		Tag:                  c.Tag,
		CallbackURL:          c.CallbackURL,
		CData:                c.CData,
	}
}

func voucherCollectOpts(c *VoucherCfg) collectOpts {
	return collectOpts{
		Collect:              c.Collect,
		CustomerPhoneNumber:  c.CustomerPhoneNumber,
		CustomerEmailAddress: c.CustomerEmailAddress,
		ServiceNumber:        c.ServiceNumber,
		CustomerName:         c.CustomerName,
		CustomerAddress:      c.CustomerAddress,
		CustomerNumber:       c.CustomerNumber,
		TRID:                 c.TRID,
		Tag:                  c.Tag,
		CallbackURL:          c.CallbackURL,
		CData:                c.CData,
	}
}

func productCollectOpts(c *ProductCfg) collectOpts {
	return collectOpts{
		Collect:              c.Collect,
		CustomerPhoneNumber:  c.CustomerPhoneNumber,
		CustomerEmailAddress: c.CustomerEmailAddress,
		ServiceNumber:        c.ServiceNumber,
		CustomerName:         c.CustomerName,
		CustomerAddress:      c.CustomerAddress,
		CustomerNumber:       c.CustomerNumber,
		TRID:                 c.TRID,
		Tag:                  c.Tag,
		CallbackURL:          c.CallbackURL,
		CData:                c.CData,
	}
}

func subscriptionCollectOpts(c *SubscriptionCfg) collectOpts {
	return collectOpts{
		Collect:              c.Collect,
		CustomerPhoneNumber:  c.CustomerPhoneNumber,
		CustomerEmailAddress: c.CustomerEmailAddress,
		ServiceNumber:        c.ServiceNumber,
		CustomerName:         c.CustomerName,
		CustomerAddress:      c.CustomerAddress,
		CustomerNumber:       c.CustomerNumber,
		TRID:                 c.TRID,
		Tag:                  c.Tag,
		CallbackURL:          c.CallbackURL,
		CData:                c.CData,
	}
}

func cashinCollectOpts(c *CashinCfg) collectOpts {
	return collectOpts{
		Collect:              c.Collect,
		CustomerPhoneNumber:  c.CustomerPhoneNumber,
		CustomerEmailAddress: c.CustomerEmailAddress,
		ServiceNumber:        c.ServiceNumber,
		CustomerName:         c.CustomerName,
		CustomerAddress:      c.CustomerAddress,
		CustomerNumber:       c.CustomerNumber,
		TRID:                 c.TRID,
		Tag:                  c.Tag,
		CallbackURL:          c.CallbackURL,
		CData:                c.CData,
	}
}

// --- Scenarios -------------------------------------------------------

func (h *Harness) scenarioPing(ctx context.Context) error {
	p, err := h.client.Verify.Ping(ctx)
	if err != nil {
		return err
	}
	if p.Version == "" {
		return errors.New("empty response")
	}
	detail("server time:    " + p.Time.Format(time.RFC3339))
	detail("server version: " + p.Version)
	detail("nonce echo:     " + p.Nonce)
	detail("public key:     " + p.Key)
	return nil
}

func (h *Harness) scenarioTokenRefresh(ctx context.Context) error {
	first, err := h.client.Tokens.AccessToken(ctx)
	if err != nil {
		return err
	}
	forced, err := h.client.Tokens.Refresh(ctx)
	if err != nil {
		return err
	}
	if forced == "" {
		return errors.New("refresh returned empty token")
	}
	if _, err := h.client.Verify.Ping(ctx); err != nil {
		return err
	}
	detail("first  bearer prefix: " + prefix(first, 12) + "...")
	detail("forced bearer prefix: " + prefix(forced, 12) + "...")
	detail(fmt.Sprintf("identical: %v", first == forced))
	return nil
}

func prefix(s string, n int) string {
	if len(s) < n {
		return s
	}
	return s[:n]
}

func (h *Harness) scenarioAccount(ctx context.Context) error {
	a, err := h.client.Verify.Account(ctx)
	if err != nil {
		return err
	}
	detail(fmt.Sprintf("agent:           %s (id=%s)", a.AgentName, a.AgentID))
	detail("company:         " + a.CompanyName)
	detail(fmt.Sprintf("balance:         %g %s", a.Balance, a.Currency))
	detail(fmt.Sprintf("daily limit max: %g", a.LimitMax))
	detail(fmt.Sprintf("limit remaining: %g", a.LimitRemaining))
	return nil
}

func (h *Harness) scenarioMerchants(ctx context.Context) error {
	ms, err := h.client.Masterdata.Merchants(ctx)
	if err != nil {
		return err
	}
	detail(fmt.Sprintf("merchants: %d", len(ms)))
	sample := 5
	if len(ms) < sample {
		sample = len(ms)
	}
	for i := 0; i < sample; i++ {
		detail(fmt.Sprintf("  - %s : %s (%s, %s)",
			ms[i].Merchant, ms[i].Name, ms[i].Country, ms[i].Status))
	}
	if len(ms) > sample {
		detail(fmt.Sprintf("  ...and %d more", len(ms)-sample))
	}
	return nil
}

func (h *Harness) scenarioServices(ctx context.Context) error {
	ss, err := h.client.Masterdata.Services(ctx)
	if err != nil {
		return err
	}
	detail(fmt.Sprintf("services: %d", len(ss)))

	byType := map[smob.ServiceType]int{}
	for _, s := range ss {
		byType[s.Type]++
	}
	types := make([]string, 0, len(byType))
	for t := range byType {
		types = append(types, string(t))
	}
	sort.Strings(types)
	detail("distribution by type:")
	for _, t := range types {
		detail(fmt.Sprintf("  - %s: %d", t, byType[smob.ServiceType(t)]))
	}
	listOfType(ss, smob.ServiceTypeVoucher, "VOUCHER services")
	listOfType(ss, smob.ServiceTypeSubscription, "SUBSCRIPTION services")
	listVerifiable(ss)
	return nil
}

func listOfType(services []smob.Service, t smob.ServiceType, label string) {
	var match []smob.Service
	for _, s := range services {
		if s.Type == t {
			match = append(match, s)
		}
	}
	if len(match) == 0 {
		return
	}
	detail(label + ":")
	for _, s := range match {
		detail(fmt.Sprintf("  - serviceId=%d merchant=%s title=%s", s.ServiceID, s.Merchant, s.Title))
	}
}

func listVerifiable(services []smob.Service) {
	var match []smob.Service
	for _, s := range services {
		if s.IsVerifiable {
			match = append(match, s)
		}
	}
	if len(match) == 0 {
		return
	}
	detail("verifiable services (isVerifiable=true) — candidates for the 'verify' block:")
	for _, s := range match {
		detail(fmt.Sprintf("  - serviceId=%d merchant=%s title=%s", s.ServiceID, s.Merchant, s.Title))
	}
}

func (h *Harness) scenarioCashout(ctx context.Context) error {
	c := h.cfg.Cashout
	if c == nil {
		return skip("no 'cashout' block in config")
	}
	items, err := h.client.Masterdata.Cashouts(ctx, c.ServiceID)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return fmt.Errorf("no cashout items for serviceId=%d", c.ServiceID)
	}
	item := items[0]
	var amt float64
	if item.AmountLocalCur() != nil {
		amt = *item.AmountLocalCur()
	}
	detail(fmt.Sprintf("picked: %s (%s, %s, local=%g %s)",
		item.PayItemID(), item.Name(), item.AmountType(), amt, item.LocalCur()))
	amount, err := resolveAmount(&item, c.Amount)
	if err != nil {
		return err
	}
	return quoteAndReport(ctx, h.client, &item, amount, cashoutCollectOpts(c))
}

func (h *Harness) scenarioBill(ctx context.Context) error {
	c := h.cfg.Bill
	if c == nil {
		return skip("no 'bill' block in config")
	}
	bills, err := h.client.Initiate.Bills(ctx, c.Merchant, c.ServiceID, c.ServiceNumber)
	if err != nil {
		return err
	}
	if len(bills) == 0 {
		return fmt.Errorf("no bills for %s/%d/%s", c.Merchant, c.ServiceID, c.ServiceNumber)
	}
	bill := bills[0]
	amount, err := resolveAmount(&bill, 0)
	if err != nil {
		return err
	}
	detail(fmt.Sprintf("picked: %s (%s, amount=%d %s, due=%s)",
		bill.PayItemID(), bill.BillType, amount, bill.LocalCur(), bill.BillDueDate.Format("2006-01-02")))
	return quoteAndReport(ctx, h.client, &bill, amount, billCollectOpts(c))
}

func (h *Harness) scenarioTopup(ctx context.Context) error {
	c := h.cfg.Topup
	if c == nil {
		return skip("no 'topup' block in config")
	}
	items, err := h.client.Masterdata.Topups(ctx, c.ServiceID)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return fmt.Errorf("no topup items for serviceId=%d", c.ServiceID)
	}
	item := items[0]
	var amt float64
	if item.AmountLocalCur() != nil {
		amt = *item.AmountLocalCur()
	}
	detail(fmt.Sprintf("picked: %s (%s, %s, local=%g %s)",
		item.PayItemID(), item.Name(), item.AmountType(), amt, item.LocalCur()))
	amount, err := resolveAmount(&item, c.Amount)
	if err != nil {
		return err
	}
	return quoteAndReport(ctx, h.client, &item, amount, topupCollectOpts(c))
}

func (h *Harness) scenarioVoucher(ctx context.Context) error {
	c := h.cfg.Voucher
	if c == nil {
		return skip("no 'voucher' block in config")
	}
	items, err := h.client.Masterdata.Vouchers(ctx, c.ServiceID)
	if err != nil {
		var pe *smob.APIError
		if errors.As(err, &pe) && pe.RespCode == 41004 {
			return skip("/v2/voucher rejects serviceId=%d (respCode 41004)", c.ServiceID)
		}
		return err
	}
	if len(items) == 0 {
		return fmt.Errorf("no vouchers for serviceId=%d", c.ServiceID)
	}
	item := items[0]
	amount, err := resolveAmount(&item, c.Amount)
	if err != nil {
		return err
	}
	detail(fmt.Sprintf("picked: %s (%s, %s)", item.PayItemID(), item.Name(), item.AmountType()))
	return quoteAndReport(ctx, h.client, &item, amount, voucherCollectOpts(c))
}

func (h *Harness) scenarioProduct(ctx context.Context) error {
	c := h.cfg.Product
	if c == nil {
		return skip("no 'product' block in config")
	}
	items, err := h.client.Masterdata.Products(ctx, c.ServiceID)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return fmt.Errorf("no products for serviceId=%d", c.ServiceID)
	}
	item := items[0]
	amount, err := resolveAmount(&item, c.Amount)
	if err != nil {
		return err
	}
	detail(fmt.Sprintf("picked: %s (%s, %s)", item.PayItemID(), item.Name(), item.AmountType()))
	return quoteAndReport(ctx, h.client, &item, amount, productCollectOpts(c))
}

func (h *Harness) scenarioSubscription(ctx context.Context) error {
	c := h.cfg.Subscription
	if c == nil {
		return skip("no 'subscription' block in config")
	}
	if c.ServiceNumber == "" && c.CustomerNumber == "" {
		return skip("subscription block needs serviceNumber or customerNumber")
	}
	subs, err := h.client.Initiate.Subscriptions(ctx, c.Merchant, c.ServiceID, c.ServiceNumber, c.CustomerNumber)
	if err != nil {
		return err
	}
	if len(subs) == 0 {
		return fmt.Errorf("no subscriptions for %s/%d (serviceNumber=%s, customerNumber=%s)",
			c.Merchant, c.ServiceID, c.ServiceNumber, c.CustomerNumber)
	}
	sub := subs[0]
	amount, err := resolveAmount(&sub, c.Amount)
	if err != nil {
		return err
	}
	detail(fmt.Sprintf("picked: %s (%s, customer=%s, due=%s)",
		sub.PayItemID(), sub.Name(), sub.CustomerName, sub.DueDate.Format("2006-01-02")))
	return quoteAndReport(ctx, h.client, &sub, amount, subscriptionCollectOpts(c))
}

func (h *Harness) scenarioCashin(ctx context.Context) error {
	c := h.cfg.Cashin
	if c == nil {
		return skip("no 'cashin' block in config")
	}
	items, err := h.client.Masterdata.Cashins(ctx, c.ServiceID)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return fmt.Errorf("no cashin items for serviceId=%d", c.ServiceID)
	}
	item := items[0]
	var amt float64
	if item.AmountLocalCur() != nil {
		amt = *item.AmountLocalCur()
	}
	detail(fmt.Sprintf("picked: %s (%s, %s, local=%g %s)",
		item.PayItemID(), item.Name(), item.AmountType(), amt, item.LocalCur()))
	amount, err := resolveAmount(&item, c.Amount)
	if err != nil {
		return err
	}
	return quoteAndReport(ctx, h.client, &item, amount, cashinCollectOpts(c))
}

func (h *Harness) scenarioVerifyServiceNumber(ctx context.Context) error {
	c := h.cfg.Verify
	if c == nil {
		return skip("no 'verify' block in config")
	}
	ok, err := h.client.AccountValidation.VerifyServiceNumber(ctx, c.Merchant, c.ServiceID, c.ServiceNumber)
	if err != nil {
		var pe *smob.APIError
		if errors.As(err, &pe) && pe.RespCode == 40408 {
			return skip("service %s/%d does not support pre-payment verification (respCode 40408)",
				c.Merchant, c.ServiceID)
		}
		return err
	}
	detail(fmt.Sprintf("%s for %s/%d -> %v", c.ServiceNumber, c.Merchant, c.ServiceID, ok))
	return nil
}

func (h *Harness) scenarioValidateAccount(ctx context.Context) error {
	c := h.cfg.Validation
	if c == nil {
		return skip("no 'validate' block in config")
	}
	ca, err := h.client.AccountValidation.ValidateAccount(ctx, c.Destination, c.ServiceID)
	if err != nil {
		var pe *smob.APIError
		if errors.As(err, &pe) && pe.HTTPStatus == 401 {
			return skip("GET /v2/validate is restricted and not enabled for this partner (HTTP 401)")
		}
		return err
	}
	detail("destination: " + ca.Destination)
	detail(fmt.Sprintf("status:      %s", ca.Status))
	detail("name:        " + ca.Name)
	return nil
}

func (h *Harness) scenarioHistoryLast7Days(ctx context.Context) error {
	today := time.Now().UTC()
	from := today.AddDate(0, 0, -7)
	rows, err := h.client.Verify.HistoryByDateRange(ctx, from, today)
	if err != nil {
		return err
	}
	detail("range:        " + from.Format("2006-01-02") + " -> " + today.Format("2006-01-02"))
	detail(fmt.Sprintf("transactions: %d", len(rows)))
	sample := 3
	if len(rows) < sample {
		sample = len(rows)
	}
	for i := 0; i < sample; i++ {
		r := rows[i]
		var price float64
		if r.PriceLocalCur != nil {
			price = *r.PriceLocalCur
		}
		detail(fmt.Sprintf("  - %s : %s, %g %s, trid=%s",
			r.PTN, r.Status, price, r.LocalCur, r.TRID))
	}
	return nil
}
