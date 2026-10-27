%{!?nli_version:%global nli_version 19.0.0}
%{!?nli_release:%global nli_release 1}
Name:           nli-controller-agent
Version:        %{nli_version}
Release:        %{nli_release}%{?dist}
Summary:        Nexora Log Intelligence Ver19 controller and Web console
License:        LicenseRef-Proprietary
URL:            https://github.com/vjay136/nexora_log_intelligence_ver19.git
Source0:        %{name}-%{version}.tar.gz
ExclusiveArch:  x86_64
Requires(pre):  shadow-utils
Requires(post): systemd coreutils sed grep
Requires(preun): systemd
Requires(postun): systemd
Requires:       python3 curl openssl hostname iproute
AutoReqProv:    no
%global debug_package %{nil}
%global _build_id_links none
%global __os_install_post %{nil}

%description
NLI v19 controller providing authenticated enrollment, fleet synchronization,
rollouts, telemetry APIs, and the byte-verified webview_ver19 console.

%prep
%autosetup -n %{name}-%{version}

%build
# The static binary and Web assets are built and verified before rpmbuild.

%install
install -d %{buildroot}%{_bindir} %{buildroot}%{_sbindir} %{buildroot}%{_sysconfdir}/nli
install -d %{buildroot}/usr/lib/systemd/system %{buildroot}%{_datadir}/nli/webview_ver19 %{buildroot}%{_datadir}/nli/supabase
install -d %{buildroot}%{_datadir}/nli/metrics
install -d %{buildroot}%{_docdir}/%{name} %{buildroot}/var/lib/nli-controller
install -m 0755 bin/nli-controller-agent bin/nli %{buildroot}%{_bindir}/
install -m 0755 sbin/nli-controller-setup %{buildroot}%{_sbindir}/nli-controller-setup
install -m 0755 sbin/nli-supabase-setup %{buildroot}%{_sbindir}/nli-supabase-setup
install -m 0755 sbin/nli-admin-reset %{buildroot}%{_sbindir}/nli-admin-reset
install -m 0644 supabase/nli_schema.sql %{buildroot}%{_datadir}/nli/supabase/nli_schema.sql
install -m 0640 configs/controller.json configs/controller.env configs/geoip.env %{buildroot}%{_sysconfdir}/nli/
install -m 0644 systemd/nli-controller-agent.service systemd/nli-controller-bootstrap.service systemd/nli-geoip-update.service systemd/nli-geoip-update.timer %{buildroot}/usr/lib/systemd/system/
cp -a web/. %{buildroot}%{_datadir}/nli/webview_ver19/
find %{buildroot}%{_datadir}/nli/webview_ver19 -type d -exec chmod 0755 {} +
find %{buildroot}%{_datadir}/nli/webview_ver19 -type f -exec chmod 0644 {} +
install -m 0644 README.md %{buildroot}%{_docdir}/%{name}/README.md
install -m 0644 V19_UPES_GPRS_ARCHITECTURE_AND_ACCEPTANCE.md V19_R7_TEST_RESULTS.md V19_R8_TEST_RESULTS.md V19_R8_QRW_OPERATOR_TEST_GUIDE.md V19_QUARANTINE_RESOLUTION_WIZARD.md NLI_V19_UPES_GPRS_ARCHITECTURE.svg %{buildroot}%{_docdir}/%{name}/
install -m 0644 metrics/README.md metrics/metric-catalog.json metrics/prometheus-scrape.yml metrics/grafana-dashboard.json %{buildroot}%{_datadir}/nli/metrics/

%pre
getent group nli >/dev/null || groupadd -r nli
getent passwd nli >/dev/null || useradd -r -g nli -d /var/lib/nli-controller -s /sbin/nologin nli

%post
config_file=%{_sysconfdir}/nli/controller.json
if grep -Eq '"web_dir"[[:space:]]*:[[:space:]]*"/usr/share/nli/webview_ver[15678]"' "$config_file"; then
  sed -i -E 's#/usr/share/nli/webview_ver[15678](["/])#/usr/share/nli/webview_ver19\1#g' "$config_file"
fi
secret_file=%{_sysconfdir}/nli/controller.env
chown root:nli "$secret_file"
chmod 0640 "$secret_file"
systemctl daemon-reload >/dev/null 2>&1 || :
systemctl enable nli-controller-bootstrap.service >/dev/null 2>&1 || :
# The GeoIP timer remains disabled until an administrator accepts the upstream
# license in /etc/nli/geoip.env and deliberately enables it.
if [ -s /etc/nli/controller-bootstrap.env ]; then systemctl start nli-controller-bootstrap.service >/dev/null 2>&1 || :; fi

%preun
if [ "$1" -eq 0 ]; then
  systemctl disable --now nli-controller-bootstrap.service nli-controller-agent.service >/dev/null 2>&1 || :
fi

%postun
systemctl daemon-reload >/dev/null 2>&1 || :

%files
%attr(0755,root,root) %{_bindir}/nli-controller-agent
%attr(0755,root,root) %{_bindir}/nli
%attr(0755,root,root) %{_sbindir}/nli-controller-setup
%attr(0755,root,root) %{_sbindir}/nli-supabase-setup
%attr(0755,root,root) %{_sbindir}/nli-admin-reset
%dir %attr(0750,root,nli) %{_sysconfdir}/nli
%config(noreplace) %attr(0640,root,nli) %{_sysconfdir}/nli/controller.json
%config(noreplace) %attr(0640,root,nli) %{_sysconfdir}/nli/controller.env
%config(noreplace) %attr(0640,root,nli) %{_sysconfdir}/nli/geoip.env
%ghost %attr(0600,root,root) %{_sysconfdir}/nli/controller-bootstrap.env
%attr(0644,root,root) /usr/lib/systemd/system/nli-controller-agent.service
%attr(0644,root,root) /usr/lib/systemd/system/nli-controller-bootstrap.service
%attr(0644,root,root) /usr/lib/systemd/system/nli-geoip-update.service
%attr(0644,root,root) /usr/lib/systemd/system/nli-geoip-update.timer
%dir %attr(0755,root,root) %{_datadir}/nli
%dir %attr(0755,root,root) %{_datadir}/nli/webview_ver19
%attr(0644,root,root) %{_datadir}/nli/webview_ver19/*
%dir %attr(0755,root,root) %{_datadir}/nli/supabase
%attr(0644,root,root) %{_datadir}/nli/supabase/nli_schema.sql
%dir %attr(0755,root,root) %{_datadir}/nli/metrics
%attr(0644,root,root) %{_datadir}/nli/metrics/README.md
%attr(0644,root,root) %{_datadir}/nli/metrics/metric-catalog.json
%attr(0644,root,root) %{_datadir}/nli/metrics/prometheus-scrape.yml
%attr(0644,root,root) %{_datadir}/nli/metrics/grafana-dashboard.json
%dir %attr(0750,nli,nli) /var/lib/nli-controller
%doc %{_docdir}/%{name}/README.md
%doc %{_docdir}/%{name}/V19_UPES_GPRS_ARCHITECTURE_AND_ACCEPTANCE.md
%doc %{_docdir}/%{name}/V19_R7_TEST_RESULTS.md
%doc %{_docdir}/%{name}/V19_R8_TEST_RESULTS.md
%doc %{_docdir}/%{name}/V19_R8_QRW_OPERATOR_TEST_GUIDE.md
%doc %{_docdir}/%{name}/V19_QUARANTINE_RESOLUTION_WIZARD.md
%doc %{_docdir}/%{name}/NLI_V19_UPES_GPRS_ARCHITECTURE.svg

%changelog
* Wed Oct 28 2026 Nexora Engineering <engineering@nexora.local> - 19.0.0-8
- Add the guided quarantine parser lifecycle, approval-gated fleet delivery, exact acknowledgement verification, replay, and safe offboarding controls.

* Mon Oct 26 2026 Nexora Engineering <engineering@nexora.local> - 19.0.0-7
- Add measured validation pipeline telemetry, NUPIRE fix, and R7 browser regressions

* Mon Oct 26 2026 Nexora Engineering <engineering@nexora.local> - 19.0.0-6
- Add bounded UPES failure discovery and the GPRS evidence-gated parser repair workflow.
- Include the UPES/GPRS architecture and operator acceptance guide in release docs.

* Sun Oct 25 2026 Nexora Engineering <engineering@nexora.local> - 19.0.0-5
- Ship the bounded client and controller metrics catalog, Prometheus example, and Grafana dashboard.

* Sun Oct 25 2026 Nexora Engineering <engineering@nexora.local> - 19.0.0-4
- Add persisted bounded telemetry buckets for the Overall Timeline dashboard
- Support revisioned single or multiple input-path profile rollouts with independent approval

* Sat Oct 24 2026 Nexora Engineering <engineering@nexora.local> - 19.0.0-3
- Add communication status, notifications, browser recovery, and guided parser remediation.

* Sat Oct 24 2026 Nexora Engineering <engineering@nexora.local> - 19.0.0-2
- Wire canonical telemetry, controller resources, replay and delivery evidence into live monitoring.

* Wed Oct 21 2026 Nexora Engineering <engineering@nexora.local> - 19.0.0-1
- Establish Ver19 release identity, Ver18 upgrade input, and documented rollback path.


* Wed Oct 21 2026 Nexora Engineering <engineering@nexora.local> - 18.0.0-1
- Fix canonical subsystem instrumentation, efficient live heartbeats, and background WebView refresh.

* Tue Oct 20 2026 Nexora Engineering <engineering@nexora.local> - 18.0.0-1
- Add NURSF canonical real-time synchronization, subsystem monitoring, and Grafana-grade dashboards.

* Mon Oct 19 2026 Nexora Engineering <engineering@nexora.local> - 16.0.0-2
- Add durable Assurance execution, parser synthesis evidence, and expanded scenarios.

* Sun Oct 18 2026 Nexora Engineering <engineering@nexora.local> - 16.0.0-1
- Add V16 Assurance quarantine analysis, official branding, live dashboards, and Vite deployment support

* Fri Oct 16 2026 Nexora Engineering <engineering@nexora.local> - 14.0.0-1
- Add AWS RHEL/Rocky wrappers, private VPC defaults, Cloudflare Tunnel, and hosted-WebApp deployment contracts

* Tue Oct 13 2026 Nexora Engineering <engineering@nexora.local> - 12.0.0-1
- Add enterprise bootstrap, protected password file, and corporate PKI setup

* Sun Oct 11 2026 Nexora Engineering <engineering@nexora.local> - 12.0.0-5
- Add public HTTPS setup modes, quarantine operations, and API monitoring

* Sat Oct 10 2026 Nexora Engineering <engineering@nexora.local> - 12.0.0-4
- Require an exact release identity and complete bundle checksum verification

* Sat Oct 10 2026 Nexora Engineering <engineering@nexora.local> - 12.0.0-3
- Validate distro-suffixed V12 RPM release metadata in guided installers

* Wed Oct 07 2026 Nexora Engineering <engineering@nexora.local> - 12.0.0-1
- Build Nexora Log Intelligence Ver12 from source with Supabase and Vercel delivery

* Tue Oct 06 2026 Nexora Engineering <engineering@nexora.local> - 5.0.0-4
- Replace the legacy cloud adapter with RLS-protected Supabase metadata RPCs
- Run Assurance through production ParserPacks and preserve the full static prefix
- Retain encrypted fixtures until manual purge and support authenticated .log download
- Keep Fleet, Parser Catalogue, and Migration Center actionable on empty collections

* Tue Oct 06 2026 Nexora Engineering <engineering@nexora.local> - 5.0.0-3
- Fail closed until the WebView verifies the controller identity and login contract
- Add explicit first-run controller setup with private-network TLS bootstrap
- Consume enrollment tokens once, support deliberate rotation, and declare hostname dependency
- Preserve cookie, CSRF, and bearer authentication through the fixed-origin Vercel proxy

* Tue Oct 06 2026 Nexora Engineering <engineering@nexora.local> - 5.0.0-2
- Repair relative WebView assets and ship production-only frontend files
- Add premium role-aware console and interactive cloud metadata setup

* Mon Oct 05 2026 Nexora Engineering <engineering@nexora.local> - 5.0.0-1
- Embed byte-verified webview_ver5 assets and hardened controller service
- Preserve controller configuration, secrets, and durable state
