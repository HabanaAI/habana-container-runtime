Name: habana-container-hook
Version: %{version}
Release: %{release}
Group: Development Tools

Vendor: Habana Labs
Packager: Habana Labs

Summary: HABANA container runtime hook
URL: https://github.com/HabanaAI/habana-container-hook
License: Apache-2.0

Source0: habana-container-hook
Source1: config.toml
Source2: oci-habana-hook
Source3: oci-habana-hook.json
Source4: LICENSE

%description
Provides a OCI hook to enable Habana device support in containers.

%prep
cp %{SOURCE0} %{SOURCE1} %{SOURCE2} %{SOURCE3} %{SOURCE4} .

%install
mkdir -p %{buildroot}%{_bindir}
install -m 755 -t %{buildroot}%{_bindir} habana-container-hook
mkdir -p %{buildroot}/etc/habana-container-runtime
install -m 644 -t %{buildroot}/etc/habana-container-runtime config.toml
mkdir -p %{buildroot}/usr/libexec/oci/hooks.d
install -m 755 -t %{buildroot}/usr/libexec/oci/hooks.d oci-habana-hook
mkdir -p %{buildroot}/usr/share/containers/oci/hooks.d
install -m 644 -t %{buildroot}/usr/share/containers/oci/hooks.d oci-habana-hook.json

%posttrans
ln -sf %{_bindir}/habana-container-hook %{_bindir}/habana-container-runtime-hook

%postun
rm -f %{_bindir}/habana-container-runtime-hook

%files
%license LICENSE
%{_bindir}/habana-container-hook

%config /etc/habana-container-runtime/config.toml
/usr/libexec/oci/hooks.d/oci-habana-hook
/usr/share/containers/oci/hooks.d/oci-habana-hook.json
