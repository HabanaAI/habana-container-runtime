Name: habana-container-runtime
Version: %{version}
Release: %{release}%{?dist}
vendor: 'Habana Labs'
Packager: Tal Cohen <tacohen@habana.ai>

BuildArch: x86_64
Summary: habana-container-runtime package
URL: https://github.com/HabanaAI/habana-container-runtime
License: Dual MIT/GPL

Source0: habana-container-runtime
Source1: LICENSE
Source2: habana-container-hook
Source3: config.toml
Source4: oci-habana-hook
Source5: oci-habana-hook.json
Source6: LICENSE
Source7: habana-container-cli

%if 0%{?suse_version}
Requires: libseccomp2
Requires: libapparmor1
%else
Requires: libseccomp
%endif

%description
HABANA container runtime
 Provides a modified version of runc allowing users to run Intel® Gaudi® enabled containers.

%prep
cp %{SOURCE0} %{SOURCE1} %{SOURCE2} %{SOURCE3} %{SOURCE4} %{SOURCE5} %{SOURCE6} %{SOURCE7} .

%install
mkdir -p %{buildroot}%{_bindir}
install -m 755 -t %{buildroot}%{_bindir} habana-container-runtime
install -m 755 -t %{buildroot}%{_bindir} habana-container-hook
install -m 755 -t %{buildroot}%{_bindir} habana-container-cli

mkdir -p %{buildroot}/etc/habana-container-runtime
install -m 644 -t %{buildroot}/etc/habana-container-runtime config.toml
mkdir -p %{buildroot}/usr/libexec/oci/hooks.d
install -m 755 -t %{buildroot}/usr/libexec/oci/hooks.d oci-habana-hook
mkdir -p %{buildroot}/usr/share/containers/oci/hooks.d
install -m 644 -t %{buildroot}/usr/share/containers/oci/hooks.d oci-habana-hook.json

%files
%license LICENSE
%{_bindir}/habana-container-runtime
%{_bindir}/habana-container-hook
%{_bindir}/habana-container-cli

%config /etc/habana-container-runtime/config.toml
/usr/libexec/oci/hooks.d/oci-habana-hook
/usr/share/containers/oci/hooks.d/oci-habana-hook.json

%clean
rm -rf %{buildroot}
