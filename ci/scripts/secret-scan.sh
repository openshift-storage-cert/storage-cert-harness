#!/usr/bin/env bash
# Secret scan: SLA files, run reports, SLA-like numerics in the MR diff, and
# editor/workspace env vars that look like API keys or tokens.
# Used by GitLab, GitHub Actions, make test, and the pre-commit hook.
#
# Numeric scan (original CI story): every added line under internal/tools/,
# internal/grader/, plans/, and fixtures/testdata — including *_test.go.
# Gate values belong only in the private KB (thresholds.json, ADR-0004).
set -euo pipefail
# shellcheck source=ci-utils.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/ci-utils.sh"

errors=0
allowlist="${CI_CONFIG_DIR}/secret-scan-allowlist.txt"
# Canonical token is secret-scan:ok; aliases match the NotebookLLM bypass names.
OVERRIDE_RE='secret-scan:ok|allow-numeric|no-gate|ignore-gate|nosecret'
OVERRIDE_MARK="secret-scan:ok"

# Spec paths: internal/tools/, internal/grader/, fixtures, plans.
# Do not exclude *_test.go / testdata — fixtures are how gate values leak into goldens.
# fixtures/ is the spec name (may be empty); testdata/ is where parser goldens live today.
SCAN_PATHS=(
	'internal/tools'
	'internal/grader'
	'fixtures'
	'plans'
	'**/testdata/**'
)

fail() {
	echo "FAIL: $*"
	errors=$((errors + 1))
}

# Strip tokens that look numeric but are not SLA bars.
strip_non_sla() {
	printf '%s\n' "$1" | sed -E \
		-e 's|https?://[^[:space:]"<>]+||g' \
		-e 's|[[:alnum:]._-]+(/[[:alnum:]._-]+)+(@sha256:[0-9a-f]+)?(:[A-Za-z0-9._+-]+)||g' \
		-e 's/\bv[0-9]+(\.[0-9]+)+\b//g' \
		-e 's/\b[0-9]+\.[0-9]+\.[0-9]+(-[A-Za-z0-9.]+)?\b//g' \
		-e 's/\bgo1\.[0-9]+(\.[0-9]+)?\b//g' \
		-e 's/\b0o[0-7]+\b//g' \
		-e 's/\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b//g' \
		-e 's/\b[0-9]+(\.[0-9]+)?[eE][+-]?[0-9]+\b//g' \
		-e 's/\b[0-9]+(\.[0-9]+)?(Ki|Mi|Gi|Ti|Pi|Ei)\b//g' \
		-e 's/[0-9]+\.[0-9]+[[:space:]]*\/[[:space:]]*[0-9]+//g' \
		-e 's/"(p[0-9]+|P[0-9]+|max|min|avg)"[[:space:]]*:[[:space:]]*[0-9]+(\.[0-9]+)?//g' \
		-e 's/\b[0-9]{4}-[0-9]{2}-[0-9]{2}\b//g' \
		-e 's/\b(TR|EX|tr|ex)-[A-Za-z0-9]+-[0-9]+\b//g' \
		-e 's/\bADR-[0-9]+\b//gi' \
		-e 's/\b[A-Z]{3,}-[0-9]{3,}\b//g' \
		-e 's|decisions/[0-9]+(,[[:space:]]*[0-9]+)*||g'
}

# Floats, or a standalone 3+ digit integer (:=8080, {"threshold":250}).
# \b is portable GNU/BSD grep -E; do not use PCRE lookbehind (?<!…).
has_sla_numeric() {
	printf '%s\n' "$1" | grep -Eq '[0-9]+\.[0-9]+|\b[0-9]{3,}\b'
}

# Testdata JSON is usually tool output (measured values). Only treat it as a
# leaked gate file when SLA/threshold vocabulary is present.
json_looks_like_thresholds() {
	local f="$1"
	[[ -f "${f}" ]] || return 1
	grep -Eiq '"sla"|"slas"|"threshold"|"thresholds"|"gate"' "${f}"
}

is_measurement_json() {
	local f="$1"
	[[ "${f}" == *.json || "${f}" == *.jsonl ]] || return 1
	[[ "${f}" == *testdata/* || "${f}" == fixtures/* || "${f}" == */fixtures/* ]]
}

line_has_override() {
	printf '%s\n' "$1" | grep -Eiq "${OVERRIDE_RE}"
}

# Same line, or the previous line in the file (YAML/Go comment token).
line_overridden() {
	local file="$1" lineno="$2" body="$3"
	if line_has_override "${body}"; then
		return 0
	fi
	if [[ -n "${file}" && -f "${file}" && "${lineno}" =~ ^[1-9][0-9]*$ && "${lineno}" -gt 1 ]]; then
		local prev
		prev="$(sed -n "$((lineno - 1))p" "${file}" 2>/dev/null || true)"
		if line_has_override "${prev}"; then
			return 0
		fi
	fi
	return 1
}

# Human-readable tokens that tripped the heuristic (from leftover text).
sla_tokens() {
	local body="$1"
	local -a toks=()
	local t
	while IFS= read -r t; do
		[[ -n "${t}" ]] || continue
		toks+=("${t}")
	done < <(printf '%s\n' "${body}" | grep -Eo '[0-9]+\.[0-9]+' || true)
	while IFS= read -r t; do
		t="${t#"${t%%[![:space:]]*}"}"
		[[ -n "${t}" ]] || continue
		toks+=("${t}")
	done < <(printf '%s\n' "${body}" | grep -Eo '\b[0-9]{3,}([.][0-9]+)?' || true)
	if [[ "${#toks[@]}" -eq 0 ]]; then
		echo "(none)"
		return
	fi
	local IFS=', '
	echo "${toks[*]}"
}

fail_sla() {
	local file="$1" lineno="$2" body="$3" leftover="$4"
	local loc="${file}"
	if [[ -n "${lineno}" ]]; then
		loc="${file}:${lineno}"
	fi
	echo "FAIL: possible SLA numeric in ${loc}"
	echo "      added line  : ${body}"
	echo "      after strip : ${leftover}"
	echo "      matched     : $(sla_tokens "${leftover}")"
	echo "      why         : floats or standalone 3+ digit integers (:=8080, JSON :250)"
	echo "                    look like copy-pasted SLA bars. Gate values belong only"
	echo "                    in the private KB (thresholds.json), never in git (ADR-0004)."
	echo "      ignored     : image refs (quay.io/…:tag), vX.Y.Z / semver, go1.N,"
	echo "                    0oNNN modes, Ki/Mi/Gi sizes, percentile keys (p50/p99),"
	echo "                    TR/ADR/Jira ids, plan names (tr-virt-010), decisions/NNNN,"
	echo "                    UUIDs, URLs, ISO dates, N.N / N ratios"
	echo "      override    : ${OVERRIDE_MARK} (or allow-numeric / ignore-gate / no-gate /"
	echo "                    nosecret) on this line or the previous line — ports, retries."
	echo "      json/other  : uncommentable false positives go in ci/secret-scan-allowlist.txt"
	errors=$((errors + 1))
}

# $1 = added line, $2 = 1 to flag / 0 to ignore, $3 = label.
selftest_case() {
	local body="$1" want="$2" label="$3"
	local leftover got=0
	if line_has_override "${body}"; then
		if [[ "${want}" -ne 0 ]]; then
			echo "SELFTEST FAIL: ${label} (override token should skip)"
			return 1
		fi
		return 0
	fi
	leftover="$(strip_non_sla "${body}")"
	if has_sla_numeric "${leftover}"; then
		got=1
	fi
	if [[ "${got}" -ne "${want}" ]]; then
		echo "SELFTEST FAIL: ${label}"
		echo "      line      : ${body}"
		echo "      leftover  : ${leftover}"
		echo "      matched   : $(sla_tokens "${leftover}")"
		echo "      expected  : $([[ "${want}" -eq 1 ]] && echo FLAG || echo ignore)"
		echo "      got       : $([[ "${got}" -eq 1 ]] && echo FLAG || echo ignore)"
		return 1
	fi
	return 0
}

secret_scan_selftest() {
	local st_err=0
	# --- pinned image refs: tag/digest, not an SLA bar ---
	selftest_case 'const DefaultImage = "quay.io/kube-burner/kube-burner:v2.8.1"' 0 "image pin vX.Y.Z" || st_err=1
	selftest_case 'const DefaultImage = "localhost/kube-burner-ocp:v-src"' 0 "local image tag" || st_err=1
	selftest_case 'const img = "registry.redhat.io/ubi10/go-toolset:1.26"' 0 "image tag two-part version" || st_err=1
	selftest_case '// pinned kube-burner v2.8.1 (ADR-0003)' 0 "version tag in comment" || st_err=1
	# --- non-SLA numeric idioms: file modes, quantities, measured percentiles ---
	selftest_case 'return os.MkdirAll(dst, 0o755)' 0 "Go octal file mode" || st_err=1
	selftest_case 'claim_size: 256Mi' 0 "k8s quantity" || st_err=1
	selftest_case 'want := map[string]float64{"p50": 6000, "p95": 7000, "p99": 8000, "max": 8200}' 0 "percentile measurements in test" || st_err=1
	selftest_case 'replicas := 50' 0 "two-digit workload param" || st_err=1
	# --- override tokens: canonical and aliases must suppress a real leak ---
	selftest_case 'port := 8080 // secret-scan:ok' 0 "comment-token override" || st_err=1
	selftest_case 'port:=8080 // allow-numeric' 0 "alias allow-numeric" || st_err=1
	selftest_case 'retries:=1000 // ignore-gate' 0 "alias ignore-gate" || st_err=1
	# --- id-shaped tokens that contain digits but aren't SLA bars ---
	selftest_case 'Provides: []string{"TR-VIRT-010"}' 0 "TR id is not an SLA" || st_err=1
	selftest_case 'name: tr-virt-010-vm-snapshot-smoke' 0 "lowercase tr id in plan name" || st_err=1
	selftest_case '// see ADR-0004 / ECOPROJECT-5274' 0 "ADR/Jira ids" || st_err=1
	selftest_case '// See decisions/0012,0008.' 0 "decisions/NNNN ADR refs" || st_err=1
	# --- real leaks: must still be caught (regression guard on strip rules above) ---
	selftest_case 'threshold := 1.5' 1 "SLA-like float" || st_err=1
	selftest_case 'maxLatency = 1500' 1 "SLA-like 3+ digit integer" || st_err=1
	selftest_case 'MaxRetries:=1000' 1 "tight Go assignment :=1000" || st_err=1
	selftest_case '{"threshold":250}' 1 "tight JSON :250" || st_err=1
	selftest_case 'const x = 100.0' 1 "SLA-like x.0 float" || st_err=1
	selftest_case 'passRatio := 0.95' 1 "SLA-like ratio" || st_err=1
	selftest_case 'Value: f64(2.0)' 1 "fake SLA pointer in tests must be marked" || st_err=1
	# --- ECOPROJECT-5404: UUID false positive: sci-notation rule must not eat the
	# UUID's first hex segment (e.g. 550e8400) before the UUID strip rule runs ---
	selftest_case 'requestID := "550e8400-e29b-41d4-a716-446655440000"' 0 "UUID whose first segment looks like sci-notation" || st_err=1
	if [[ "${st_err}" -ne 0 ]]; then
		echo "error: secret-scan --self-test failed"
		return 1
	fi
	echo "secret-scan --self-test ok"
	return 0
}

if [[ "${1:-}" == "--self-test" ]]; then
	secret_scan_selftest
	exit $?
fi

if [[ -n "${PRE_COMMIT:-}" ]]; then
	ci_log_init "pre-commit-secret-scan"
else
	ci_log_init "secret-scan"
fi

echo "==> matcher self-test"
secret_scan_selftest

# --- tracked forbidden filenames ---
echo "==> tracked-files  (refuse thresholds.json / report.json / report.md)"
tracked_hits=0
while IFS= read -r f; do
	base="$(basename "${f}")"
	case "${base}" in
	thresholds.json)
		fail "tracked real thresholds file: ${f}"
		echo "      why: real SLA bars must not be in git (ADR-0004); keep thresholds.json gitignored"
		tracked_hits=$((tracked_hits + 1))
		;;
	report.json | report.md)
		fail "tracked harness run report: ${f}"
		echo "      why: reports can contain graded numbers; do not commit report.json / report.md"
		tracked_hits=$((tracked_hits + 1))
		;;
	esac
done < <(git ls-files)
echo "    tracked-files: ${tracked_hits} issue(s)"

# --- SLA-like numerics in the MR diff (tools, grader, plans, tests, fixtures) ---
base="$(merge_base)"
echo "==> SLA-numeric  merge_base=${base:0:12}  HEAD=$(git_sha)"
echo "    paths: ${SCAN_PATHS[*]}  (includes *_test.go, testdata/, fixtures/)"
echo "    cmd: git diff -U0 ${base:0:12}...HEAD -- ${SCAN_PATHS[*]}"
echo "    override: ${OVERRIDE_RE}  (same line or previous line)"
diff_out="$(git diff -U0 "${base}"...HEAD -- "${SCAN_PATHS[@]}" || true)"
added_lines=0
overridden=0
allowlisted=0
sla_hits=0
skipped_json=0
cur_file="(unknown)"
new_line=0
if [[ -z "${diff_out}" ]]; then
	echo "    no diff under scan paths vs merge_base; skipping"
else
	while IFS= read -r line || [[ -n "${line}" ]]; do
		case "${line}" in
		"diff --git "* | "index "* | "--- "* | "\\ No newline"*)
			continue
			;;
		+++*)
			cur_file="${line#+++ }"
			cur_file="${cur_file#b/}"
			cur_file="${cur_file%%$'\t'*}"
			if [[ "${cur_file}" == /dev/null ]]; then
				cur_file="(deleted)"
			fi
			continue
			;;
		@@*)
			if [[ "${line}" =~ \+([0-9]+) ]]; then
				new_line="${BASH_REMATCH[1]}"
			fi
			continue
			;;
		esac
		if [[ "${line}" == +* ]]; then
			body="${line#+}"
			added_lines=$((added_lines + 1))
			loc_line="${new_line}"
			new_line=$((new_line + 1))
			if line_overridden "${cur_file}" "${loc_line}" "${body}"; then
				overridden=$((overridden + 1))
				continue
			fi
			if [[ -f "${allowlist}" ]] && grep -Fqx "${body}" "${allowlist}" 2>/dev/null; then
				allowlisted=$((allowlisted + 1))
				continue
			fi
			if is_measurement_json "${cur_file}" && ! json_looks_like_thresholds "${cur_file}"; then
				# Measured goldens / tool output — not a thresholds document.
				skipped_json=$((skipped_json + 1))
				continue
			fi
			leftover="$(strip_non_sla "${body}")"
			if has_sla_numeric "${leftover}"; then
				fail_sla "${cur_file}" "${loc_line}" "${body}" "${leftover}"
				sla_hits=$((sla_hits + 1))
			fi
			continue
		fi
		if [[ "${line}" == " "* ]]; then
			new_line=$((new_line + 1))
		fi
	done <<<"${diff_out}"
	echo "    scanned ${added_lines} added line(s); ${OVERRIDE_MARK}=${overridden}; allowlisted=${allowlisted}; measurement-json skipped=${skipped_json}; flagged=${sla_hits}"
fi

# --- editor / workspace env tokens ---
echo "==> editor-env  (.vscode / *.code-workspace / .cursor / .devcontainer / .env)"
secret_name_re='(api[_-]?key|token|secret|password|passwd|credential|auth|bearer|private[_-]?key|access[_-]?key|HARNESS_THRESHOLDS)'
secret_val_re='(glpat-|ghp_|gho_|github_pat_|sk-|AKIA|eyJ[A-Za-z0-9_-]{10,})'

scan_editor_file() {
	local f="$1"
	[[ -f "${f}" ]] || return 0
	git ls-files --error-unmatch "${f}" >/dev/null 2>&1 || return 0
	if grep -Eiq "${secret_name_re}" "${f}"; then
		# empty / placeholder values are allowed
		if grep -Ei "${secret_name_re}" "${f}" | grep -Evq '""|'"''"'|<set-locally>|changeme|YOUR_.*HERE|placeholder'; then
			if grep -Eq "${secret_val_re}" "${f}"; then
				fail "token-shaped value in editor/workspace file: ${f}"
				echo "      why: looks like glpat-/ghp_/sk-/AKIA/JWT; unset or use a placeholder"
				return
			fi
			if grep -Ei "${secret_name_re}" "${f}" | grep -Eq ':[[:space:]]*"[^"<][^"]{7,}"'; then
				fail "possible secret env assignment in ${f}"
				echo "      why: secret-looking name with a non-empty quoted value (not a known placeholder)"
			fi
		fi
	fi
	if grep -Eq "${secret_val_re}" "${f}"; then
		fail "token-shaped value in editor/workspace file: ${f}"
		echo "      why: file contains glpat-/ghp_/sk-/AKIA/JWT-shaped text"
	fi
}

while IFS= read -r f; do
	scan_editor_file "${f}"
done < <(git ls-files -- '.vscode/*.json' '*.code-workspace' '.cursor/**' '.devcontainer/**' '.env' '.env.*' ':!.env.example' || true)

if command -v gitleaks >/dev/null 2>&1; then
	echo "==> gitleaks detect --no-git --source ${REPO_ROOT}"
	gitleaks detect --no-git --source "${REPO_ROOT}" --verbose || fail "gitleaks reported findings (see gitleaks output above)"
else
	echo "==> gitleaks not installed; skipping (optional, see docs/ci.md)"
fi

if [[ "${errors}" -gt 0 ]]; then
	echo "--- secret-scan summary: ${errors} issue(s) ---"
	echo "    re-run: ./ci/secret-scan.sh"
	echo "    matcher: ./ci/secret-scan.sh --self-test"
	echo "    override: ${OVERRIDE_MARK} (or allow-numeric / ignore-gate / no-gate / nosecret)"
	die "secret-scan found ${errors} issue(s)"
fi
echo "secret-scan ok"
