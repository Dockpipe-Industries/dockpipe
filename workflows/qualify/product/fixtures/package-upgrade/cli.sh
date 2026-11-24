#!/bin/sh
set -eu
umask 077

mode=$1
version=$2
work=${3:-}

if [ "$mode" = project ]; then
    export HOME="$work/home" XDG_STATE_HOME="$work/state" XDG_CACHE_HOME="$work/cache"
    export DOCKPIPE_GLOBAL_ROOT="$work/global" DOCKPIPE_SYSTEM_ROOT="$work/system"
    cd "$work/project"

    if [ "$4" = initialize ]; then
        test ! -e dockpipe.config.json
        dockpipe init
        cat > workflow.yml <<'YAML'
name: installed-package-qualification
vault: none
docker_preflight: false
steps:
  - id: record-installed-version
    kind: host
    cwd: repo
    run: record.sh
YAML
        printf '%s\n' '#!/usr/bin/env bash' 'set -euo pipefail' \
            'dockpipe --version >> qualification-versions.txt' > record.sh
        chmod 755 record.sh
        dockpipe workflow validate workflow.yml
        sha256sum dockpipe.config.json workflow.yml record.sh > "$work/project.sha256"
    fi

    test "$(id -u)" -ne 0
    executable=$(command -v dockpipe)
    printf 'Normal user CLI: %s (uid %s)\n' "$executable" "$(id -u)"
    test "$(readlink -f "$executable")" = /usr/bin/dockpipe
    test "$(dockpipe --version)" = "$version"
    sha256sum /usr/bin/dockpipe
    sha256sum -c "$work/project.sha256"
    dockpipe --workflow-file "$PWD/workflow.yml" --workdir "$PWD"
    cat qualification-versions.txt
    exit 0
fi

test "$mode" = install
release=$3
prior=$4
current=$5
prior_sha=$6
current_sha=$7

. /etc/os-release
test "$ID" = "$version"
test "$(uname -m)" = x86_64
case "$ID" in
    amzn)
        test "$VERSION_ID" = 2023
        test "$(rpm -q --qf '%{VERSION}' system-release)" = "$release"
        ;;
    *) test "$VERSION_ID" = "$release" ;;
esac
printf 'Platform: %s %s %s\n' "$ID" "$release" "$(uname -m)"
test -z "$(command -v dockpipe)"

packages() {
    case "$ID" in
        alpine) apk --no-progress "$@" ;;
        rhel)
            dnf -y --setopt=timeout=20 --setopt=retries=1 --disablerepo='*' \
                --enablerepo=ubi-9-baseos-rpms --enablerepo=ubi-9-appstream-rpms "$@"
            ;;
        rocky)
            dnf -y --setopt=timeout=20 --setopt=retries=1 --disablerepo='*' --enablerepo='qualification-*' \
                --repofrompath=qualification-baseos,"https://dl.rockylinux.org/vault/rocky/$release/BaseOS/x86_64/os/" \
                --repofrompath=qualification-appstream,"https://dl.rockylinux.org/vault/rocky/$release/AppStream/x86_64/os/" \
                --setopt=qualification-baseos.gpgcheck=1 --setopt=qualification-appstream.gpgcheck=1 \
                --setopt=qualification-baseos.gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-Rocky-9 \
                --setopt=qualification-appstream.gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-Rocky-9 "$@"
            ;;
        amzn) dnf -y --setopt=timeout=20 --setopt=retries=1 --releasever="$release" "$@" ;;
    esac
}

installed_version() {
    expected=$1
    if [ "$ID" = alpine ]; then
        owner=$(apk info --who-owns /usr/bin/dockpipe)
        printf '%s\n' "$owner"
        case "$owner" in
            "/usr/bin/dockpipe is owned by dockpipe-$expected-r"[0-9]*) ;;
            *) exit 1 ;;
        esac
    else
        test "$(rpm -qf --qf '%{NAME}' /usr/bin/dockpipe)" = dockpipe
        test "$(rpm -q --qf '%{VERSION}' dockpipe)" = "$expected"
        rpm -q dockpipe
    fi
}

if [ "$ID" = alpine ]; then
    extension=apk
    packages add bash git
else
    extension=rpm
fi
printf '%s  %s\n' "$prior_sha" "/fixture/previous.$extension" \
    "$current_sha" "/fixture/current.$extension" | sha256sum -c -

upgrade_work=$(mktemp -d)
clean_work=$(mktemp -d)
for directory in "$upgrade_work" "$clean_work"; do
    mkdir "$directory/home" "$directory/project"
    chown -R nobody "$directory"
done

if [ "$ID" = alpine ]; then
    packages add --allow-untrusted /fixture/previous.apk
else
    packages install /fixture/previous.rpm
fi
installed_version "$prior"
su -s /bin/sh nobody -c "/bin/sh /fixture/cli.sh project $prior $upgrade_work initialize"
test "$(cat "$upgrade_work/project/qualification-versions.txt")" = "$prior"

if [ "$ID" = alpine ]; then
    packages add --upgrade --allow-untrusted /fixture/current.apk
else
    packages upgrade /fixture/current.rpm
fi
installed_version "$current"
test "$(cat "$upgrade_work/project/qualification-versions.txt")" = "$prior"
su -s /bin/sh nobody -c "/bin/sh /fixture/cli.sh project $current $upgrade_work resume"
printf '%s\n%s\n' "$prior" "$current" > "$upgrade_work/expected"
expected_digest=$(sha256sum "$upgrade_work/expected" | cut -d ' ' -f 1)
actual_digest=$(sha256sum "$upgrade_work/project/qualification-versions.txt" | cut -d ' ' -f 1)
test "$actual_digest" = "$expected_digest"
printf 'Workflow version history matched exactly: %s, %s\n' "$prior" "$current"

if [ "$ID" = alpine ]; then
    packages del dockpipe
    test ! -e /usr/bin/dockpipe
    packages add --allow-untrusted /fixture/current.apk
else
    packages remove dockpipe
    test ! -e /usr/bin/dockpipe
    packages install /fixture/current.rpm
fi
installed_version "$current"
su -s /bin/sh nobody -c "/bin/sh /fixture/cli.sh project $current $clean_work initialize"
test "$(cat "$clean_work/project/qualification-versions.txt")" = "$current"
printf 'Clean candidate install and workflow matched: %s\n' "$current"

if [ "$ID" = rhel ]; then
    printf 'Installed UBI RPM inventory:\n'
    rpm -qa --qf '%{NAME}-%{VERSION}-%{RELEASE}.%{ARCH}\n' | LC_ALL=C sort
fi
