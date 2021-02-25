Name: habana-container-runtime
Version: %{version}
Release: %{release}
Group: Development Tools

Vendor: Habana Labs
Packager: Habana Labs

Summary: HABANA container runtime
URL: https://github.com/HabanaAI/habana-container-runtime
# runc NOTICE file: https://github.com/opencontainers/runc/blob/master/NOTICE
License: ASL 2.0

Source0: habana-container-runtime
Source1: LICENSE

%if 0%{?suse_version}
Requires: libseccomp2
Requires: libapparmor1
%else
Requires: libseccomp
%endif

%description
Provides a modified version of runc allowing users to run GPU enabled
containers.

%prep
cp %{SOURCE0} %{SOURCE1} .

%install
mkdir -p %{buildroot}%{_bindir}
install -m 755 -t %{buildroot}%{_bindir} habana-container-runtime

%files
%license LICENSE
%{_bindir}/habana-container-runtime
