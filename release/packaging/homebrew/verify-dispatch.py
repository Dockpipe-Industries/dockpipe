#!/usr/bin/env python3
"""Admit Homebrew notification only after a successful staging publication."""
import json
import os
from pathlib import Path
import re
import subprocess


PUBLICATION_JOB = "staging-release / Publish staging candidate"


def published_staging(run, jobs, repository):
    if (
        run.get("repository", {}).get("full_name") != repository
        or run.get("head_repository", {}).get("full_name") != repository
        or run.get("path") != ".github/workflows/ci.yml"
        or run.get("event") != "push"
        or run.get("head_branch") != "staging"
        or run.get("status") != "completed"
        or run.get("conclusion") != "success"
    ):
        return False

    publications = [job for job in jobs if job.get("name") == PUBLICATION_JOB]
    return len(publications) == 1 and all(
        job.get("status") == "completed" and job.get("conclusion") == "success"
        for job in publications
    )


def github(path, paginated=False):
    command = ["gh", "api", path]
    if paginated:
        command.extend(["--paginate", "--slurp"])
    return json.loads(subprocess.check_output(command, text=True))


def main():
    repository = os.environ["GITHUB_REPOSITORY"]
    run_id = os.environ["SOURCE_RUN_ID"]
    attempt = os.environ.get("SOURCE_RUN_ATTEMPT", "")
    if not re.fullmatch(r"[1-9][0-9]*", run_id):
        raise ValueError("Expected a numeric staging CI run ID")
    if attempt and not re.fullmatch(r"[1-9][0-9]*", attempt):
        raise ValueError("Expected a numeric staging CI attempt")

    run_path = f"repos/{repository}/actions/runs/{run_id}"
    if attempt:
        run_path += f"/attempts/{attempt}"
    run = github(run_path)
    # Bind the jobs to this exact attempt, including for manual recovery.
    jobs_path = f"repos/{repository}/actions/runs/{run_id}/attempts/{run['run_attempt']}/jobs"
    pages = github(jobs_path, paginated=True)
    jobs = [job for page in pages for job in page["jobs"]]
    published = published_staging(run, jobs, repository)
    with Path(os.environ["GITHUB_OUTPUT"]).open("a") as output:
        output.write(f"published={str(published).lower()}\n")
    if not published:
        print("No successful staging publication in this CI attempt; tap notification skipped.")


if __name__ == "__main__":
    main()
