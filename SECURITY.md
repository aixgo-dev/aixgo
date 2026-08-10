# Security policy

## Reporting a vulnerability

Report security issues privately. Go to the **Security** tab of this repository and choose **Report a vulnerability**. That opens a private advisory only the maintainers can see. Do not open a public issue for a suspected vulnerability.

We aim to acknowledge a report within 3 business days.

## Supported versions

aixgo is pre-1.0 and the API still moves. Security fixes land on the default branch and ship in the next tagged release. Older tags are not patched, so run a recent release.

## In scope

aixgo is a library and a binary you run in your own infrastructure. It reads provider credentials from your environment, calls the LLM providers named in your config, and can do whatever the tools you wire up can do. Within that, we treat the following as vulnerabilities:

- Bypasses of the controls in `pkg/security`: the SSRF guard, prompt injection filter, input sanitizer, rate limiter, auth and IAP checks, and log redaction.
- Config parsing. A YAML file should not be able to make aixgo read files, open connections, or run code that the config does not describe.
- Credential handling. Provider keys and tokens must not leak into logs, traces, audit records, or model prompts.
- The published release artifacts and their checksums.

## Out of scope

- A model returning wrong, biased, or harmful text. aixgo does not vouch for provider output.
- Prompt injection that only changes what a model says. What we act on is injected text reaching a tool, a credential, or a network destination it should not.
- Tools you write yourself. If your tool shells out or reads arbitrary paths, that is a bug in your tool. `docs/SECURITY_BEST_PRACTICES.md` covers how to avoid it.
- Consequences of your own deployment choices, such as exposing the HTTP surface without auth or handing an agent a key with more access than it needs.
- Vulnerabilities in the LLM providers themselves. Report those to the provider.
