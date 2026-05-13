'use client';

import React, { useEffect, useMemo, useRef, useState } from 'react';
import './GrafanaDashboard.css';

const DEFAULT_GRAFANA_PATH = '/grafana/dashboards';

const isGrafanaEditorPath = (grafanaUrl) => {
    return (
        grafanaUrl.pathname === '/grafana/dashboard/new' ||
        grafanaUrl.searchParams.has('editPanel')
    );
};

const buildDefaultGrafanaPath = (deviceId = null) => {
    const params = new URLSearchParams({
        theme: 'light',
        orgId: '1',
        refresh: '10s',
        kiosk: '1',
        ...(deviceId ? { 'var-deviceId': deviceId } : {}),
    });

    return `${DEFAULT_GRAFANA_PATH}?${params.toString()}`;
};

const getGrafanaPathWithParams = (path, deviceId = null) => {
    const grafanaUrl = new URL(path, window.location.origin);

    if (!grafanaUrl.pathname.startsWith('/grafana')) {
        return path;
    }

    if (!grafanaUrl.searchParams.has('theme')) {
        grafanaUrl.searchParams.set('theme', 'light');
    }

    if (!grafanaUrl.searchParams.has('orgId')) {
        grafanaUrl.searchParams.set('orgId', '1');
    }

    if (isGrafanaEditorPath(grafanaUrl)) {
        grafanaUrl.searchParams.delete('kiosk');
        grafanaUrl.searchParams.delete('refresh');
    } else {
        if (!grafanaUrl.searchParams.has('refresh')) {
            grafanaUrl.searchParams.set('refresh', '10s');
        }

        grafanaUrl.searchParams.set('kiosk', '1');
    }

    if (deviceId) {
        grafanaUrl.searchParams.set('var-deviceId', deviceId);
    } else {
        grafanaUrl.searchParams.delete('var-deviceId');
    }

    return `${grafanaUrl.pathname}${grafanaUrl.search}${grafanaUrl.hash}`;
};

const GrafanaDashboard = ({ deviceId = null }) => {
    const iframeRef = useRef(null);
    const [iframeSrc, setIframeSrc] = useState('');

    const defaultSrc = useMemo(() => {
        return buildDefaultGrafanaPath(deviceId);
    }, [deviceId]);

    useEffect(() => {
        const url = new URL(window.location.href);
        const savedGrafanaPath = url.searchParams.get('grafana');

        if (savedGrafanaPath && savedGrafanaPath.startsWith('/grafana')) {
            setIframeSrc(getGrafanaPathWithParams(savedGrafanaPath, deviceId));
        } else {
            setIframeSrc(defaultSrc);
        }
    }, [defaultSrc, deviceId]);

    useEffect(() => {
        const url = new URL(window.location.href);
        const savedGrafanaPath = url.searchParams.get('grafana');

        if (!savedGrafanaPath || !savedGrafanaPath.startsWith('/grafana')) {
            setIframeSrc(defaultSrc);
        }
    }, [deviceId, defaultSrc]);

    useEffect(() => {
        const syncIframeFromParentUrl = () => {
            const url = new URL(window.location.href);
            const savedGrafanaPath = url.searchParams.get('grafana');

            if (savedGrafanaPath && savedGrafanaPath.startsWith('/grafana')) {
                setIframeSrc(getGrafanaPathWithParams(savedGrafanaPath, deviceId));
                return;
            }

            setIframeSrc(defaultSrc);
        };

        window.addEventListener('popstate', syncIframeFromParentUrl);

        return () => {
            window.removeEventListener('popstate', syncIframeFromParentUrl);
        };
    }, [defaultSrc, deviceId]);

    useEffect(() => {
        let interval = null;
        let crossOriginBlocked = false;

        const syncParentUrlWithIframe = () => {
            if (crossOriginBlocked) return;

            try {
                const iframeWindow = iframeRef.current?.contentWindow;
                if (!iframeWindow) return;

                const currentPath =
                    iframeWindow.location.pathname +
                    iframeWindow.location.search +
                    iframeWindow.location.hash;

                if (!currentPath.startsWith('/grafana')) return;

                const sanitizedPath = getGrafanaPathWithParams(currentPath, deviceId);

                if (sanitizedPath !== currentPath) {
                    setIframeSrc(sanitizedPath);
                    return;
                }

                const parentUrl = new URL(window.location.href);
                const existingGrafanaPath = parentUrl.searchParams.get('grafana');

                if (existingGrafanaPath !== sanitizedPath) {
                    parentUrl.searchParams.set('grafana', sanitizedPath);
                    window.history.pushState({}, '', parentUrl.toString());
                }
            } catch {
                crossOriginBlocked = true;
                if (interval) clearInterval(interval);
                interval = null;
            }
        };

        interval = setInterval(syncParentUrlWithIframe, 300);

        return () => {
            if (interval) clearInterval(interval);
        };
    }, [deviceId]);

    const hideGrafanaChrome = () => {
        try {
            const iframe = iframeRef.current;
            const doc = iframe?.contentDocument || iframe?.contentWindow?.document;
            const iframeWindow = iframe?.contentWindow;

            if (!doc || !iframeWindow) return;

            const styleId = 'taksa-hide-grafana-chrome';
            let style = doc.getElementById(styleId);

            if (!style) {
                style = doc.createElement('style');
                style.id = styleId;
                doc.head.appendChild(style);
            }

            style.textContent = `
                [data-testid="sidemenu"],
                [data-testid="sidemenu-toggle"],
                [data-testid="nav-toggle"],
                [data-testid="menu-toggle"],
                [data-testid="mega-menu"],
                [data-testid="mega-menu-drawer"],
                [data-testid="navigation-menu"],
                [data-testid="main-navigation"],
                [aria-label="Main menu"],
                [aria-label="Main navigation"],
                nav[aria-label*="navigation" i],
                nav[aria-label*="main" i],
                aside[aria-label*="navigation" i],
                aside[aria-label*="menu" i],
                [class*="sidemenu"],
                [class*="SideMenu"],
                [class*="mega-menu"],
                [class*="MegaMenu"],
                [class*="NavMenu"],
                [class*="navigation-menu"],
                [class*="MainNavigation"] {
                    display: none !important;
                    visibility: hidden !important;
                    pointer-events: none !important;
                }

                button[aria-label*="menu" i],
                button[aria-label*="navigation" i],
                button[aria-label*="main menu" i],
                button[aria-label*="open menu" i],
                header button[aria-label*="menu" i],
                header button[aria-label*="navigation" i],
                [role="banner"] button[aria-label*="menu" i],
                [role="banner"] button[aria-label*="navigation" i] {
                    display: none !important;
                    visibility: hidden !important;
                    pointer-events: none !important;
                }

                header a[href="/grafana"],
                header a[href="/grafana/"],
                [role="banner"] a[href="/grafana"],
                [role="banner"] a[href="/grafana/"],
                header a[aria-label*="Home" i],
                [role="banner"] a[aria-label*="Home" i],
                header [aria-label*="Grafana" i],
                [role="banner"] [aria-label*="Grafana" i],
                [data-testid="grafana-logo"],
                [class*="grafana-logo"],
                [class*="GrafanaLogo"] {
                    display: none !important;
                    visibility: hidden !important;
                    pointer-events: none !important;
                }

                a[href*="/grafana/login"],
                a[href*="/login"],
                [aria-label*="sign in" i],
                [aria-label*="Sign in" i],
                [title*="Sign in" i],
                [title*="sign in" i] {
                    display: none !important;
                    visibility: hidden !important;
                    pointer-events: none !important;
                }

                [class*="powered-by"],
                a[href*="grafana.com"],
                [role="contentinfo"] {
                    display: none !important;
                    visibility: hidden !important;
                    pointer-events: none !important;
                }
            `;

            const hideElement = (element) => {
                if (!element) return;

                element.style.display = 'none';
                element.style.visibility = 'hidden';
                element.style.pointerEvents = 'none';
            };

            const hideOpenSideNavigation = () => {
                const navLinks = Array.from(
                    doc.querySelectorAll(
                        [
                            'a[href*="/grafana/dashboards"]',
                            'a[href*="/grafana/explore"]',
                            'a[href*="/grafana/alerting"]',
                            'a[href*="/grafana/connections"]',
                            'a[href*="/grafana/admin"]',
                            'a[href*="/dashboards"]',
                            'a[href*="/explore"]',
                            'a[href*="/alerting"]',
                            'a[href*="/connections"]',
                            'a[href*="/admin"]',
                        ].join(',')
                    )
                );

                navLinks.forEach((link) => {
                    let element = link.parentElement;
                    let drawerElement = null;
                    let overlayElement = null;

                    while (element && element !== doc.body) {
                        const rect = element.getBoundingClientRect();
                        const computedStyle = iframeWindow.getComputedStyle(element);
                        const role = element.getAttribute('role') || '';
                        const ariaModal = element.getAttribute('aria-modal') || '';

                        if (
                            rect.width >= 220 &&
                            rect.width <= 700 &&
                            rect.height >= iframeWindow.innerHeight * 0.5 &&
                            rect.left <= 160
                        ) {
                            drawerElement = element;
                        }

                        if (
                            (
                                role === 'dialog' ||
                                ariaModal === 'true' ||
                                computedStyle.position === 'fixed'
                            ) &&
                            rect.height >= iframeWindow.innerHeight * 0.5 &&
                            rect.left <= 10
                        ) {
                            overlayElement = element;
                        }

                        element = element.parentElement;
                    }

                    hideElement(drawerElement);

                    if (
                        overlayElement &&
                        overlayElement !== doc.body &&
                        overlayElement !== doc.documentElement
                    ) {
                        hideElement(overlayElement);
                    }
                });
            };

            const hideGrafanaLogo = () => {
                const headerElements = Array.from(
                    doc.querySelectorAll('header *, [role="banner"] *')
                );

                headerElements.forEach((element) => {
                    const rect = element.getBoundingClientRect();
                    const href = element.getAttribute('href') || '';
                    const ariaLabel = element.getAttribute('aria-label') || '';
                    const title = element.getAttribute('title') || '';

                    const isLogoLike =
                        href === '/grafana' ||
                        href === '/grafana/' ||
                        ariaLabel.toLowerCase().includes('grafana') ||
                        title.toLowerCase().includes('grafana');

                    if (
                        isLogoLike &&
                        rect.left <= 120 &&
                        rect.top <= 80
                    ) {
                        hideElement(element.closest('a') || element);
                    }
                });
            };

            const hideSignIn = () => {
                const signInElements = Array.from(
                    doc.querySelectorAll(
                        [
                            'a[href*="/grafana/login"]',
                            'a[href*="/login"]',
                            '[aria-label*="sign in" i]',
                            '[title*="sign in" i]',
                            'header *',
                            '[role="banner"] *',
                        ].join(',')
                    )
                );

                signInElements.forEach((element) => {
                    const text = element.textContent?.trim().toLowerCase() || '';
                    const href = element.getAttribute('href') || '';
                    const ariaLabel = element.getAttribute('aria-label') || '';
                    const title = element.getAttribute('title') || '';
                    const rect = element.getBoundingClientRect();

                    const isSignIn =
                        text === 'sign in' ||
                        href.includes('/login') ||
                        href.includes('/grafana/login') ||
                        ariaLabel.toLowerCase().includes('sign in') ||
                        title.toLowerCase().includes('sign in');

                    if (
                        isSignIn &&
                        rect.top <= 120
                    ) {
                        const control =
                            element.closest('a') ||
                            element.closest('button') ||
                            element.closest('[role="button"]') ||
                            element;

                        hideElement(control);
                    }
                });
            };

            hideOpenSideNavigation();
            hideGrafanaLogo();
            hideSignIn();
        } catch (error) {
            console.warn('Could not hide Grafana chrome:', error);
        }
    };

    useEffect(() => {
        const iframe = iframeRef.current;
        if (!iframe) return;

        let observer = null;
        let menuInterval = null;

        const applyIframeTweaks = () => {
            hideGrafanaChrome();
        };

        const handleLoad = () => {
            applyIframeTweaks();

            try {
                const doc = iframe.contentDocument || iframe.contentWindow?.document;
                if (!doc?.body) return;

                if (observer) observer.disconnect();

                observer = new MutationObserver(() => {
                    applyIframeTweaks();
                });

                observer.observe(doc.body, {
                    childList: true,
                    subtree: true,
                    attributes: true,
                });

                if (menuInterval) clearInterval(menuInterval);
                menuInterval = setInterval(applyIframeTweaks, 500);
            } catch (error) {
                console.warn('Could not observe Grafana DOM:', error);
            }
        };

        iframe.addEventListener('load', handleLoad);

        return () => {
            iframe.removeEventListener('load', handleLoad);
            if (observer) observer.disconnect();
            if (menuInterval) clearInterval(menuInterval);
        };
    }, [iframeSrc]);

    if (!iframeSrc) return null;

    return (
        <div className="grafana-wrapper">
            <div className="grafana-frame-container">
                <iframe
                    ref={iframeRef}
                    key={deviceId || 'default'}
                    src={iframeSrc}
                    title="Grafana Dashboard"
                    className="grafana-iframe"
                    frameBorder="0"
                    allowFullScreen
                    onLoad={hideGrafanaChrome}
                />
            </div>
        </div>
    );
};

export default GrafanaDashboard;