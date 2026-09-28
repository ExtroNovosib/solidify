#!/usr/bin/env python3
"""Read-only, per-declaration Tunnel calibration using current source spans."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import tarfile
import tempfile


ROOT = Path(__file__).resolve().parents[1]
MANIFEST = ROOT / "testdata/tunnel_calibration/manifest.json"
RUNS = []
CONFIGS = {}
CACHE_DIR = None


def command(args, cwd, output=None):
    result = subprocess.run(args, cwd=cwd, check=False, capture_output=True, text=True)
    RUNS.append({"args": list(map(str,args)), "cwd": str(cwd), "exitCode": result.returncode, "stderr": result.stderr})
    if result.returncode:
        raise RuntimeError(f"{' '.join(map(str, args))} failed ({result.returncode}):\n{result.stderr}\n{result.stdout}")
    if output:
        output.write_text(result.stdout)
    return result.stdout


def target_state(source):
    checkout = Path(command(["git", "rev-parse", "--show-toplevel"], source).strip())
    names = command(["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z"], checkout).split("\0")
    digest = hashlib.sha256()
    for name in sorted(filter(None, names)):
        path = checkout / name
        if path.is_file():
            digest.update(name.encode())
            digest.update(path.read_bytes())
    return {"head": command(["git", "rev-parse", "HEAD"], checkout).strip(),
            "status": command(["git", "status", "--porcelain"], checkout), "sha256": digest.hexdigest()}


def checks(site):
    value = site["check"]
    return value if isinstance(value, list) else [value]


def evidence_fields(issue):
    return dict(part.split("=", 1) for part in issue.get("evidence", "").split(":",1)[-1].split(";") if "=" in part)


def matching(issue, site, declarations):
    if issue["id"] not in checks(site):
        return False
    if site.get("identityContains") and site["identityContains"] not in issue.get("identity", "") + issue.get("evidence", ""):
        return False
    fields = evidence_fields(issue)
    if issue["id"]=="SOLID-I/constructor-role" and site["kind"]=="parameter" and fields.get("parameter")!=site["symbol"]:
        return False
    if site["kind"] == "parameter" and issue["id"] == "SOLID-D/concrete-dependency":
        # Existing constructor diagnostics retain function+canonical dependency
        # identity and can start at the signature rather than the parameter.
        dependency = site.get("type", "").lstrip("*...").rsplit(".", 1)[-1]
        return issue["file"] == site["path"] and fields.get("function") == site["owner"] and fields.get("dependency", "").rsplit(".", 1)[-1] == dependency
    if site["kind"] == "field" and fields.get("field") and fields["field"] != site["symbol"]:
        return False
    locations = [{"file": issue["file"], "line": issue["line"]}]
    if issue["id"] == "SOLID-O/discriminator-dispatch":
        locations += issue.get("relatedLocations", [])
    return any(loc["file"] == site["path"] and site["line"] <= loc["line"] <= site["endLine"] for loc in locations)


def validate_evidence(site, issues, declarations):
    if not issues:
        return []
    issue = issues[0]
    fields = evidence_fields(issue)
    errors = []
    if site["family"] in ("CP-05", "CP-06", "CP-16"):
        for method in site.get("fixtureEvidence", {}).get("methods", []):
            if method not in fields.get("methods", "").split(","):
                errors.append(f"missing used method {method}")
    if site["family"] == "CP-04":
        if fields.get("parameter") != site["symbol"]:
            errors.append("constructor parameter identity differs")
        for key in ("broad", "narrow"):
            if sorted(fields.get(key, "").split(",")) != sorted(site["fixtureEvidence"][key]):
                errors.append(f"{key} method set differs")
        if not issue.get("relatedLocations"):
            errors.append("missing narrow stored-field location")
    if site["family"] in ("CP-01", "CP-18") and len(issue.get("relatedLocations", [])) < 2:
        errors.append("missing contract/workflow related evidence")
    if site["family"] in ("CP-14", "CP-15") and "enum:" not in issue.get("evidence", ""):
        errors.append("missing canonical typed enum evidence")
    if site["family"] in ("CP-12", "CP-13"):
        symbols = {symbol for group in issue.get("groups", []) for symbol in group["symbols"]}
        context = [s["symbol"] for s in declarations if s["family"] == site["family"] and s["role"] == "source-evidence"]
        for symbol in context:
            if symbol not in symbols:
                errors.append(f"missing owned policy/transport group method {symbol}")
    return errors


def scan(binary, source, output, name, config=None, profile="all", cached=False):
    args = [str(binary), "check", "-analysis=types", "-profile=" + profile, "-format=json", "-fail=false", "-quiet", "-cache=" + str(cached).lower(), "-cache-dir=" + str(CACHE_DIR)]
    if config:
        args += ["-config=" + str(config)]
    args += [str(source / "internal")]
    data = json.loads(command(args, source, output / (name + ".json")))
    CONFIGS[name] = command(args[:-1]+["-print-config",args[-1]],source,output / (name+"-config.json"))
    for issue in data:
        if Path(issue["file"]).is_absolute() or issue["schemaVersion"] != 3 or issue["fingerprintVersion"] != 4 or len(issue["fingerprint"]) != 64:
            raise RuntimeError("report path/schema/fingerprint contract changed")
    return data


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--before-binary", type=Path, help="optional already-built unchanged HEAD binary")
    args = parser.parse_args()
    global CACHE_DIR
    source, output = args.source.resolve(), args.output.resolve()
    if output == source or source in output.parents:
        raise RuntimeError("output must remain outside the read-only target")
    output.mkdir(parents=True, exist_ok=True)
    CACHE_DIR=Path(tempfile.mkdtemp(prefix="cache-",dir=output))
    manifest = json.loads(MANIFEST.read_text())
    state = target_state(source)
    if state["head"] != manifest["sourceHead"]:
        raise RuntimeError("target HEAD differs from reviewed source; refresh the reviewed manifest")
    current = output / "solidlint-after"
    command(["go", "build", "-o", str(current), "./cmd/solidlint"], ROOT)
    with tempfile.TemporaryDirectory(prefix="solidify-tunnel-before-") as temporary:
        before = args.before_binary
        if not before:
            archive = Path(temporary) / "head.tar"
            with archive.open("wb") as stream:
                subprocess.run(["git", "archive", "HEAD"], cwd=ROOT, stdout=stream, check=True)
            baseline_root = Path(temporary) / "source"
            baseline_root.mkdir()
            with tarfile.open(archive) as tar:
                tar.extractall(baseline_root, filter="data")
            before = Path(temporary) / "solidlint-before"
            command(["go", "build", "-o", str(before), "./cmd/solidlint"], baseline_root)
        before_all = scan(before, source, output, "before-all")
        stable = scan(current, source, output, "after-stable", profile="stable")
        after_all = scan(current, source, output, "after-all")
        execution = output / "execution.yml"
        execution.write_text("enabled_rules: [S, O, L, I, D]\nisp:\n  execution_methods: [TryClaimRuleUse]\n")
        architecture = ROOT / "testdata/calibration/tunnel-server.yml"
        http = output / "http.yml"
        http.write_text("enabled_rules: [S, O, L, I, D]\narchitecture:\n  logic_packages:\n    - github.com/fortunnels/tunnel/internal/webhookgateway/application/command\n    - github.com/fortunnels/tunnel/internal/webhookgateway/application/transform\n    - github.com/fortunnels/tunnel/internal/webhooksecurity/application/query\n  implementation_packages: [github.com/fortunnels/tunnel/internal/webhookinbox/adapter/http]\n  composition_roots: [github.com/fortunnels/tunnel/internal/server]\n")
        lanes = {"all": after_all, "execution": scan(current, source, output, "after-execution", execution), "architecture": scan(current, source, output, "after-architecture", architecture), "http": scan(current, source, output, "after-http", http)}
        cold = scan(current, source, output, "after-cache-cold", execution, cached=True)
        warm = scan(current, source, output, "after-cache-warm", execution, cached=True)
        if cold != warm or cold != lanes["execution"]:
            raise RuntimeError("cold/warm/disabled cache semantic findings differ")
        inventory = json.loads(command(["go", "run", str(ROOT / "scripts/tunnel-source-inventory.go"), str(source)], ROOT, output / "source-declarations.json"))
        results, errors = [], []
        sites = manifest["sites"]
        for site in sites:
            current_decls = [d for d in inventory if all(d.get(key, "") == site.get(key, "") for key in ("package", "kind", "symbol", "owner")) and (site["kind"] != "assertion" or d["type"] == site["type"])]
            if len(current_decls)>1:
                same_path=[d for d in current_decls if d["path"]==site["path"]]
                if len(same_path)==1: current_decls=same_path
            if len(current_decls) != 1:
                errors.append(site["id"] + ": source declaration missing/ambiguous")
                continue
            live = {**site, **current_decls[0]}
            lane = "execution" if site["family"] == "CP-16" else "architecture" if site["family"] in ("CP-11", "CN-08") else "http" if site["family"] == "CN-09" else "all"
            found = [issue for issue in lanes[lane] if matching(issue, live, sites)]
            previous = [issue for issue in before_all if matching(issue, live, sites)]
            ordinary = [issue for issue in stable if matching(issue, live, sites)]
            expected = site["role"] in ("primary", "cluster-member")
            problems = []
            if expected:
                if site["expect"] == "present" and not found:
                    problems.append("missing precise positive")
                if site["expect"] == "absent" and found:
                    problems.append("adjudicated false positive")
                if site["expect"] == "present":
                    problems += validate_evidence(site, found, sites)
            errors.extend(site["id"] + ": " + problem for problem in problems)
            results.append({"id": site["id"], "family": site["family"], "role": site["role"], "lane": lane, "expect": site["expect"], "source": current_decls[0], "before": previous, "stable": ordinary, "after": found, "verdict": "FAIL" if problems else "PASS" if expected else "SOURCE_EVIDENCE"})
        statistics=json.loads(command([str(current),"stats","-analysis=types","-profile=all","-format=json","-cache=false","-quiet",str(source / "internal")],source,output / "after-stats.json"))
        if not statistics["packages"] or not all(pkg["typeComplete"] for pkg in statistics["packages"]):
            errors.append("strict types stats contain incomplete packages")
        coverage={entry["check"]:entry for entry in statistics["checkCoverage"]}
        if coverage["SOLID-D/layer-import"]["status"]!="configuration-unavailable":
            errors.append("architecture coverage without policy is not configuration-unavailable")
        for check in ("SOLID-L/discarded-read","SOLID-L/noop-deadline","SOLID-I/constructor-role","SOLID-S/transport-workflow","SOLID-D/concrete-dependency"):
            if coverage[check]["status"]!="complete": errors.append(check+": typed coverage incomplete")
        configured_statistics=json.loads(command([str(current),"stats","-analysis=types","-profile=all","-format=json","-cache=false","-quiet","-config="+str(architecture),str(source / "internal")],source,output / "after-architecture-stats.json"))
        configured_coverage={entry["check"]:entry for entry in configured_statistics["checkCoverage"]}
        if not all(pkg["typeComplete"] for pkg in configured_statistics["packages"]):errors.append("configured stats contain incomplete types")
        for check in ("SOLID-D/layer-import","SOLID-D/wiring-outside-root"):
            if configured_coverage[check]["status"]!="complete":errors.append(check+": configured architecture coverage incomplete")
        baseline_path=output / "demonstration-baseline.json"
        baseline_path.unlink(missing_ok=True)
        command([str(current),"baseline","init","-analysis=types","-profile=all","-cache=false","-baseline="+str(baseline_path),"-baseline-reason=temporary calibration filter demonstration",str(source / "internal")],source)
        baseline_filtered=json.loads(command([str(current),"check","-analysis=types","-profile=all","-cache=false","-format=json","-fail=false","-baseline="+str(baseline_path),str(source / "internal")],source,output / "after-baseline-filtered.json"))
        if baseline_filtered: errors.append("temporary baseline did not filter its own accepted findings")
        annotations=[]
        for path in (source / "internal").rglob("*.go"):
            for number,line in enumerate(path.read_text().splitlines(),1):
                if "solidlint:ignore" in line or "solidify:ignore" in line: annotations.append({"path":str(path.relative_to(source)),"line":number,"directive":line.strip()})
        metadata={"analysisMode":"types", "statistics":{"unconfigured":statistics,"architecture":configured_statistics}, "commands":RUNS, "effectiveConfigs":CONFIGS,"solidifySource":{"head":command(["git","rev-parse","HEAD"],ROOT).strip(),"trackedDiffSHA256":hashlib.sha256(command(["git","diff","--binary"],ROOT).encode()).hexdigest(),"untrackedAnalyzerSHA256":{str(path.relative_to(ROOT)):hashlib.sha256(path.read_bytes()).hexdigest() for path in (ROOT / "internal").rglob("*.go") if path.name in ("lsp_contract.go","isp_constructor.go","ocp_enum.go","srp_transport_workflow.go")}},"beforeBuildSource":"unchanged tracked Solidify HEAD via git archive" if not args.before_binary else "caller-provided unchanged binary","beforeBinarySHA256":hashlib.sha256(before.read_bytes()).hexdigest(),"currentBinarySHA256":hashlib.sha256(current.read_bytes()).hexdigest(),"currentVersion":command([str(current),"-version"],ROOT).strip(),"beforeVersion":command([str(before),"-version"],ROOT).strip(),"baselineDemonstration":{"path":str(baseline_path),"rawCount":len(after_all),"filteredCount":len(baseline_filtered),"installedInTarget":False},"sourceSuppressions":annotations,"calibrationReportsUseBaseline":False,"architectureWithoutPolicy":coverage["SOLID-D/layer-import"]}
        final = target_state(source)
        if state != final:
            errors.append("target source/config/baseline state changed")
        summary = {"metadata": metadata, "source": str(source), "targetBefore": state, "targetAfter": final, "cacheParity": cold == warm == lanes["execution"], "positiveSites": sum(r["expect"] == "present" and r["role"] in ("primary", "cluster-member") for r in results), "negativeControls": sum(r["expect"] == "absent" and r["role"] == "primary" for r in results), "sourceEvidenceRows": sum(r["verdict"] == "SOURCE_EVIDENCE" for r in results), "errors": errors, "results": results}
        (output / "comparison.json").write_text(json.dumps(summary, indent=2) + "\n")
        if errors:
            raise RuntimeError("\n".join(errors))
        print(f"PASS: {summary['positiveSites']} precise positives, {summary['negativeControls']} controls, 28 families; cache parity and unchanged target verified. Evidence: {output / 'comparison.json'}")


if __name__ == "__main__":
    main()
