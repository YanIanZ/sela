package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/admin"
)

// cmdAdmin provisions operator identities out of band. The anonymous user
// journey never creates staff, and the console has no self-signup route.
func cmdAdmin(ctx context.Context, args []string, in io.Reader, out io.Writer) (int, error) {
	if len(args) == 0 {
		return 2, errors.New("admin: create or revoke")
	}
	switch args[0] {
	case "create":
		return cmdAdminCreate(ctx, args[1:], in, out)
	case "create-partner":
		return cmdAdminCreatePartner(ctx, args[1:], in, out)
	case "revoke":
		return cmdAdminRevoke(ctx, args[1:], out)
	}
	return 2, fmt.Errorf("admin: unknown subcommand %q (create, create-partner or revoke)", args[0])
}

func cmdAdminCreate(ctx context.Context, args []string, in io.Reader, out io.Writer) (int, error) {
	fs := flag.NewFlagSet("admin create", flag.ContinueOnError)
	fs.SetOutput(out)
	username := fs.String("username", "", "operator username (3-32 lowercase characters)")
	role := fs.String("role", "", "owner|security_admin|directory_editor|directory_reviewer|research_editor|safety_reviewer|ops")
	org := fs.String("org", "", "optional partner organisation label")
	if err := fs.Parse(args); err != nil {
		return 2, err
	}
	if *username == "" || *role == "" {
		return 2, errors.New("admin create: --username and --role are required; the password is read from stdin")
	}
	if !admin.ValidRole(admin.Role(*role)) {
		return 2, fmt.Errorf("admin create: unknown role %q", *role)
	}
	raw, err := io.ReadAll(io.LimitReader(in, 4097))
	if err != nil {
		return 1, err
	}
	password := strings.TrimRight(string(raw), "\r\n")
	if len(raw) > 4096 || password == "" {
		return 2, errors.New("admin create: provide one password on stdin (max 4096 bytes)")
	}
	cfg, pool, err := opsDatabase(ctx, true)
	if err != nil {
		return exitFor(err), err
	}
	defer pool.Close()
	if len(cfg.AdminMFAKey) != 32 {
		return 1, errors.New("SELA_ADMIN_MFA_KEY is not set (base64 of exactly 32 bytes)")
	}
	acctCfg := accountConfig(cfg, opsLogger(io.Discard))
	hasher, err := account.NewHasher(acctCfg.Hash, acctCfg.HashConcurrency, acctCfg.HashWait)
	if err != nil {
		return 1, err
	}
	st, err := admin.New(pool, admin.Options{Hasher: hasher, MFAKey: cfg.AdminMFAKey, Now: time.Now})
	if err != nil {
		return 1, err
	}
	p, secret, err := st.Create(ctx, *username, password, admin.Role(*role), *org)
	if err != nil {
		if errors.Is(err, admin.ErrInvalid) {
			return 2, fmt.Errorf("admin create: %w (check the username and password policy)", err)
		}
		return 1, err
	}
	fmt.Fprintf(out, "created operator %q with role %q\n", p.Username, p.Role)
	printTOTP(out, p.Username, secret)
	return 0, nil
}

func printTOTP(out io.Writer, username, secret string) {
	otpauth := url.URL{
		Scheme: "otpauth",
		Host:   "totp",
		Path:   "/Sela:" + username,
	}
	q := otpauth.Query()
	q.Set("secret", secret)
	q.Set("issuer", "Sela")
	q.Set("algorithm", "SHA1")
	q.Set("digits", "6")
	q.Set("period", "30")
	otpauth.RawQuery = q.Encode()
	fmt.Fprintf(out, "Add this TOTP secret to the operator's authenticator now; it is shown once:\n%s\n%s\n", secret, otpauth.String())
}

// cmdAdminCreatePartner provisions an organisation-scoped partner account
// (D-076) after the operator has checked the person and, for a
// practitioner, their licence. The password is read from stdin.
func cmdAdminCreatePartner(ctx context.Context, args []string, in io.Reader, out io.Writer) (int, error) {
	fs := flag.NewFlagSet("admin create-partner", flag.ContinueOnError)
	fs.SetOutput(out)
	username := fs.String("username", "", "login name (lower case, 3-32 characters)")
	role := fs.String("role", "", "partner_admin|practitioner|partner_staff")
	org := fs.String("org", "", "care organisation id (from the operator console)")
	name := fs.String("display-name", "", "name shown to people choosing help (required for practitioners)")
	ptype := fs.String("professional-type", "", "psychologist|psychiatrist|doctor|counsellor|nurse|social_worker (practitioners)")
	license := fs.String("license", "", "optional licence number (STR/SIP), verified later by another operator")
	if err := fs.Parse(args); err != nil {
		return 2, err
	}
	if *username == "" || *role == "" || *org == "" {
		return 2, errors.New("admin create-partner: --username, --role and --org are required; the password is read from stdin")
	}
	if !admin.IsPartnerRole(admin.Role(*role)) {
		return 2, fmt.Errorf("admin create-partner: role %q is not a partner role", *role)
	}
	raw, err := io.ReadAll(io.LimitReader(in, 4097))
	if err != nil {
		return 1, err
	}
	password := strings.TrimRight(string(raw), "\r\n")
	if len(raw) > 4096 || password == "" {
		return 2, errors.New("admin create-partner: provide one password on stdin (max 4096 bytes)")
	}
	cfg, pool, err := opsDatabase(ctx, true)
	if err != nil {
		return exitFor(err), err
	}
	defer pool.Close()
	if len(cfg.AdminMFAKey) != 32 {
		return 1, errors.New("SELA_ADMIN_MFA_KEY is not set (base64 of exactly 32 bytes)")
	}
	acctCfg := accountConfig(cfg, opsLogger(io.Discard))
	hasher, err := account.NewHasher(acctCfg.Hash, acctCfg.HashConcurrency, acctCfg.HashWait)
	if err != nil {
		return 1, err
	}
	st, err := admin.New(pool, admin.Options{Hasher: hasher, MFAKey: cfg.AdminMFAKey, Now: time.Now})
	if err != nil {
		return 1, err
	}
	p, secret, err := st.CreatePartner(ctx, admin.PartnerInput{Username: *username, Password: password, Role: admin.Role(*role),
		CareOrgID: *org, DisplayName: *name, ProfessionalType: *ptype, LicenseNo: *license})
	if err != nil {
		if errors.Is(err, admin.ErrInvalid) {
			return 2, fmt.Errorf("admin create-partner: %w (check the organisation id, role fields and password policy)", err)
		}
		return 1, err
	}
	fmt.Fprintf(out, "created partner account %q with role %q for organisation %s\n", p.Username, p.Role, p.CareOrgID)
	printTOTP(out, p.Username, secret)
	return 0, nil
}

func cmdAdminRevoke(ctx context.Context, args []string, out io.Writer) (int, error) {
	fs := flag.NewFlagSet("admin revoke", flag.ContinueOnError)
	fs.SetOutput(out)
	username := fs.String("username", "", "operator username")
	reason := fs.String("reason", "operator revoked", "short audit reason")
	if err := fs.Parse(args); err != nil {
		return 2, err
	}
	if *username == "" {
		return 2, errors.New("admin revoke: --username is required")
	}
	cfg, pool, err := opsDatabase(ctx, true)
	if err != nil {
		return exitFor(err), err
	}
	defer pool.Close()
	if len(cfg.AdminMFAKey) != 32 {
		return 1, errors.New("SELA_ADMIN_MFA_KEY is not set (base64 of exactly 32 bytes)")
	}
	acctCfg := accountConfig(cfg, opsLogger(io.Discard))
	hasher, err := account.NewHasher(acctCfg.Hash, acctCfg.HashConcurrency, acctCfg.HashWait)
	if err != nil {
		return 1, err
	}
	st, err := admin.New(pool, admin.Options{Hasher: hasher, MFAKey: cfg.AdminMFAKey, Now: time.Now})
	if err != nil {
		return 1, err
	}
	if err := st.Revoke(ctx, nil, *username, *reason); err != nil {
		if errors.Is(err, admin.ErrInvalid) {
			return 2, fmt.Errorf("admin revoke: unknown operator %q", *username)
		}
		return 1, err
	}
	fmt.Fprintf(out, "revoked operator %q and its live sessions\n", *username)
	return 0, nil
}
