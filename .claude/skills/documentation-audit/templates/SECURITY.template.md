# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| [latest]| :white_check_mark: |
| [older] | :x:                |

[Update this table with your actual version support policy]

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please report them via one of these methods:

### Preferred: GitHub Security Advisories

1. Go to the [Security tab](https://github.com/[org]/[project]/security)
2. Click "Report a vulnerability"
3. Fill out the form with details

### Alternative: Email

Send an email to [security@example.com] with:

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if you have one)

### What to Include

To help us understand and address the issue quickly, please include:

- **Type of issue**: (e.g., SQL injection, XSS, authentication bypass)
- **Full paths** of source files related to the issue
- **Location** of affected source code (tag/branch/commit/direct URL)
- **Configuration** required to reproduce
- **Step-by-step instructions** to reproduce
- **Proof-of-concept or exploit code** (if possible)
- **Impact** of the issue

## Response Timeline

- **Initial Response**: Within 48 hours
- **Status Update**: Within 7 days
- **Fix Timeline**: Depends on severity
  - **Critical**: Expedited fix within days
  - **High**: Fix within 30 days
  - **Medium/Low**: Fix in next regular release

## Security Update Process

1. **Triage**: We assess severity and impact
2. **Development**: We develop and test a fix
3. **Disclosure**: We coordinate disclosure with reporter
4. **Release**: We release patched version
5. **Announcement**: We publish security advisory

## Public Disclosure

We follow coordinated vulnerability disclosure:

- We'll work with you on disclosure timeline
- We prefer 90-day disclosure window after fix
- We'll credit you in security advisory (if you wish)
- We may request CVE assignment for critical issues

## Security Best Practices

When using this project:

- **Keep dependencies updated**: Run `go get -u` regularly
- **Review security advisories**: Watch this repository for updates
- **Use latest version**: Older versions may have known vulnerabilities
- **Report responsibly**: Follow this policy for reporting issues
- **Audit your config**: Ensure secure configuration in production

## Security Features

[Document any security features your project provides, e.g.:]

- Authentication mechanisms
- Authorization controls
- Encryption support
- Input validation
- Rate limiting
- Audit logging

## Known Security Considerations

[Document any known security limitations or considerations, e.g.:]

- This tool requires X permissions
- Data is stored in Y format (encrypted/plain)
- Network communication uses Z protocol

## Security Hall of Fame

We appreciate security researchers who help keep our project secure:

- [Researcher Name] - [Vulnerability Type] - [Date]
- [Add researchers who report vulnerabilities]

## Contact

Security Team: [security@example.com]

PGP Key: [If applicable, include PGP key fingerprint or link]

---

Thank you for helping keep [Project Name] secure!
