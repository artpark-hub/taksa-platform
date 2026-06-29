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

const normalizeGrafanaTheme = (theme) => theme === 'dark' ? 'dark' : 'light';

const getStoredTheme = () => {
    if (typeof window === 'undefined') {
        return 'light';
    }

    return normalizeGrafanaTheme(
        document.documentElement.dataset.theme ||
        window.localStorage.getItem('taksa_theme') ||
        'light'
    );
};

const buildDefaultGrafanaPath = (deviceId = null, theme = 'light') => {
    const params = new URLSearchParams({
        theme: normalizeGrafanaTheme(theme),
        orgId: '1',
        refresh: '10s',
        kiosk: '1',
        ...(deviceId ? { 'var-deviceId': deviceId } : {}),
    });

    return `${DEFAULT_GRAFANA_PATH}?${params.toString()}`;
};

const getGrafanaPathWithParams = (path, deviceId = null, theme = 'light') => {
    const grafanaUrl = new URL(path, window.location.origin);

    if (!grafanaUrl.pathname.startsWith('/grafana')) {
        return path;
    }

    grafanaUrl.searchParams.set('theme', normalizeGrafanaTheme(theme));

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
    const [theme, setTheme] = useState('light');


    useEffect(() => {
        const syncTheme = (event) => {
            const nextTheme = normalizeGrafanaTheme(event?.detail?.theme || getStoredTheme());
            setTheme(nextTheme);
        };

        syncTheme();

        window.addEventListener('taksa-theme-change', syncTheme);
        window.addEventListener('storage', syncTheme);

        return () => {
            window.removeEventListener('taksa-theme-change', syncTheme);
            window.removeEventListener('storage', syncTheme);
        };
    }, []);

    const defaultSrc = useMemo(() => {
        return buildDefaultGrafanaPath(deviceId, theme);
    }, [deviceId, theme]);

    useEffect(() => {
        const url = new URL(window.location.href);
        const savedGrafanaPath = url.searchParams.get('grafana');

        if (savedGrafanaPath && savedGrafanaPath.startsWith('/grafana')) {
            const nextGrafanaPath = getGrafanaPathWithParams(savedGrafanaPath, deviceId, theme);
            setIframeSrc(nextGrafanaPath);

            if (nextGrafanaPath !== savedGrafanaPath) {
                url.searchParams.set('grafana', nextGrafanaPath);
                window.history.replaceState({}, '', url.toString());
            }
        } else {
            setIframeSrc(defaultSrc);
        }
    }, [defaultSrc, deviceId, theme]);

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
                setIframeSrc(getGrafanaPathWithParams(savedGrafanaPath, deviceId, theme));
                return;
            }

            setIframeSrc(defaultSrc);
        };

        window.addEventListener('popstate', syncIframeFromParentUrl);

        return () => {
            window.removeEventListener('popstate', syncIframeFromParentUrl);
        };
    }, [defaultSrc, deviceId, theme]);

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

                const sanitizedPath = getGrafanaPathWithParams(currentPath, deviceId, theme);

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
    }, [deviceId, theme]);

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
                [data-testid="mega-menu"],
                [data-testid="mega-menu-drawer"],
                [data-testid="navigation-menu"],
                [data-testid="main-navigation"],
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
                    width: 0 !important;
                    min-width: 0 !important;
                    max-width: 0 !important;
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

                button[aria-label*="help" i],
                button[title*="help" i],
                [aria-label*="help" i],
                [title*="help" i],
                [data-testid*="help" i],
                header button[aria-label*="help" i],
                header button[title*="help" i],
                [role="banner"] button[aria-label*="help" i],
                [role="banner"] button[title*="help" i] {
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

                main,
                [role="main"],
                [data-testid="main-view"],
                [data-testid="page-container"],
                [class*="main-view"],
                [class*="MainView"],
                [class*="page-container"],
                [class*="PageContainer"],
                [class*="Layout"] {
                    margin-left: 0 !important;
                    padding-left: 0 !important;
                    left: 0 !important;
                    width: 100% !important;
                    max-width: none !important;
                }
            `;

            const hideElement = (element) => {
                if (!element) return;

                element.style.display = 'none';
                element.style.visibility = 'hidden';
                element.style.pointerEvents = 'none';
                element.style.width = '0';
                element.style.minWidth = '0';
                element.style.maxWidth = '0';
            };

            const resetMainLayout = () => {
                const layoutElements = Array.from(
                    doc.querySelectorAll(
                        [
                            'main',
                            '[role="main"]',
                            '[data-testid="main-view"]',
                            '[data-testid="page-container"]',
                            '[class*="main-view"]',
                            '[class*="MainView"]',
                            '[class*="page-container"]',
                            '[class*="PageContainer"]',
                            '[class*="Layout"]',
                        ].join(',')
                    )
                );

                layoutElements.forEach((element) => {
                    element.style.marginLeft = '0';
                    element.style.paddingLeft = '0';
                    element.style.left = '0';
                    element.style.width = '100%';
                    element.style.maxWidth = 'none';
                });

                iframeWindow.dispatchEvent(new Event('resize'));
            };

            const hideTopLeftNavigationButtons = () => {
                const buttons = Array.from(
                    doc.querySelectorAll(
                        [
                            'header button',
                            '[role="banner"] button',
                            'button[aria-label*="navigation" i]',
                            'button[aria-label*="main menu" i]',
                            'button[aria-label*="open menu" i]',
                        ].join(',')
                    )
                );

                buttons.forEach((button) => {
                    const rect = button.getBoundingClientRect();
                    const ariaLabel = button.getAttribute('aria-label')?.toLowerCase() || '';
                    const title = button.getAttribute('title')?.toLowerCase() || '';

                    const isTopLeftNavButton =
                        rect.top <= 120 &&
                        rect.left <= 120 &&
                        (
                            ariaLabel.includes('menu') ||
                            ariaLabel.includes('navigation') ||
                            title.includes('menu') ||
                            title.includes('navigation')
                        );

                    if (isTopLeftNavButton) {
                        hideElement(button);
                    }
                });
            };

            const collapseAndHideOpenSideNavigation = () => {
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
                            rect.left <= 180
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

                    if (!drawerElement) return;

                    const closeButton = Array.from(
                        drawerElement.querySelectorAll('button, [role="button"]')
                    ).find((button) => {
                        const text = button.textContent?.trim().toLowerCase() || '';
                        const ariaLabel = button.getAttribute('aria-label')?.toLowerCase() || '';
                        const title = button.getAttribute('title')?.toLowerCase() || '';
                        const rect = button.getBoundingClientRect();

                        return (
                            rect.top <= 180 &&
                            rect.left <= 360 &&
                            (
                                text === '×' ||
                                text === 'x' ||
                                ariaLabel.includes('close') ||
                                ariaLabel.includes('collapse') ||
                                ariaLabel.includes('hide') ||
                                title.includes('close') ||
                                title.includes('collapse') ||
                                title.includes('hide')
                            )
                        );
                    });

                    if (closeButton) {
                        closeButton.click();
                    }

                    hideElement(drawerElement);

                    if (
                        overlayElement &&
                        overlayElement !== doc.body &&
                        overlayElement !== doc.documentElement
                    ) {
                        hideElement(overlayElement);
                    }

                    resetMainLayout();

                    setTimeout(() => {
                        hideElement(drawerElement);
                        resetMainLayout();
                    }, 100);
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

            const hideHelpButton = () => {
                const helpElements = Array.from(
                    doc.querySelectorAll(
                        [
                            '[aria-label*="help" i]',
                            '[title*="help" i]',
                            '[data-testid*="help" i]',
                            'header *',
                            '[role="banner"] *',
                        ].join(',')
                    )
                );

                helpElements.forEach((element) => {
                    const text = element.textContent?.trim().toLowerCase() || '';
                    const ariaLabel = element.getAttribute('aria-label') || '';
                    const title = element.getAttribute('title') || '';
                    const dataTestId = element.getAttribute('data-testid') || '';
                    const rect = element.getBoundingClientRect();

                    const isHelp =
                        text === '?' ||
                        text === 'help' ||
                        ariaLabel.toLowerCase().includes('help') ||
                        title.toLowerCase().includes('help') ||
                        dataTestId.toLowerCase().includes('help');

                    if (
                        isHelp &&
                        rect.top <= 120
                    ) {
                        const control =
                            element.closest('button') ||
                            element.closest('a') ||
                            element.closest('[role="button"]') ||
                            element;

                        hideElement(control);
                    }
                });
            };

            collapseAndHideOpenSideNavigation();
            resetMainLayout();
            hideTopLeftNavigationButtons();
            hideGrafanaLogo();
            hideSignIn();
            hideHelpButton();
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
                    key={`${deviceId || 'default'}-${theme}`}
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