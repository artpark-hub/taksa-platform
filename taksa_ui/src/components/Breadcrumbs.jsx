import React from 'react';
import { useLocation, Link } from 'react-router-dom';
import { ChevronRight } from 'lucide-react';
import './DashboardLayout.css';

const Breadcrumbs = () => {
    const location = useLocation();
    const currentPath = location.pathname;

    // 1. DEFINE CUSTOM PARENTS HERE
    // This forces specific paths regardless of the URL structure
    const customHierarchy = {
        '/visualise': [
            { name: 'Edge Devices', link: '/data-flow' },
            { name: 'Mitsubishi PLC', link: null }, // No link, just text
            { name: 'Visualize', link: null } // Current Page
        ],
        '/InstanceDetails': [
            { name: 'Edge Devices', link: '/data-flow' },
            { name: 'Mitsubishi PLC', link: null }, // No link, just text
            { name: 'Device Details', link: null } // Current Page
        ]
    };

    // 2. CHECK IF CURRENT PAGE HAS A CUSTOM HIERARCHY
    if (customHierarchy[currentPath]) {
        return (
            <div className="header-left-breadcrumb">
                <div className="breadcrumb-container">
                    {customHierarchy[currentPath].map((item, index) => (
                        <React.Fragment key={index}>
                            {index > 0 && <ChevronRight size={16} className="breadcrumb-separator" />}

                            {item.link ? (
                                <Link to={item.link} className="breadcrumb-link">
                                    {item.name}
                                </Link>
                            ) : (
                                <span className="breadcrumb-current">{item.name}</span>
                            )}
                        </React.Fragment>
                    ))}
                </div>
            </div>
        );
    }

    // 3. FALLBACK: AUTO-GENERATE FOR OTHER PAGES (Like /data-flow, /settings)
    const pathnames = currentPath.split('/').filter((x) => x);
    const routeNameMap = {
        'data-flow': 'Edge Devices',
        'instances': 'Factory Floor Devices',
        'visualise': 'Visualize',
        'InstanceDetails': 'Device Details',
        'settings': 'Settings'
    };

    return (
        <div className="header-left-breadcrumb">
            <div className="breadcrumb-container">
                {pathnames.map((value, index) => {
                    const isLast = index === pathnames.length - 1;
                    const to = `/${pathnames.slice(0, index + 1).join('/')}`;
                    const displayName = routeNameMap[value] || value.charAt(0).toUpperCase() + value.slice(1);

                    return (
                        <React.Fragment key={to}>
                            {index > 0 && <ChevronRight size={16} className="breadcrumb-separator" />}
                            {isLast ? (
                                <span className="breadcrumb-current">{displayName}</span>
                            ) : (
                                <Link to={to} className="breadcrumb-link">{displayName}</Link>
                            )}
                        </React.Fragment>
                    );
                })}
            </div>
        </div>
    );
};

export default Breadcrumbs;