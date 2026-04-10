import React from 'react';

export default function CopilotPage() {
    return (
        <section
            style={{
                minHeight: '78vh',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                padding: '12px',
                fontFamily: 'system-ui, -apple-system, sans-serif'
            }}
        >
            <div style={{ 
                display: 'flex', 
                flexDirection: 'column', 
                alignItems: 'center', 
                textAlign: 'center' 
            }}>
                <div style={{
                    width: '50px',
                    height: '50px',
                    backgroundColor: '#ce2c31', 
                    borderRadius: '50%',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    marginBottom: '18px'
                }}>
                    <svg 
                        width="35" 
                        height="35" 
                        viewBox="0 0 24 24" 
                        fill="none" 
                        stroke="#ffffff" 
                        strokeWidth="3" 
                        strokeLinecap="round" 
                        strokeLinejoin="round"
                    >
                        <line x1="18" y1="6" x2="6" y2="18"></line>
                        <line x1="6" y1="6" x2="18" y2="18"></line>
                    </svg>
                </div>
                <h1 style={{
                    color: '#555555',
                    fontSize: '1.55rem',
                    fontWeight: 'bold',
                    margin: '0 0 14px 0',
                    letterSpacing: '-0.2px'
                }}>
                    Access Denied
                </h1>

                <p style={{
                    color: '#777777',
                    fontSize: '0.92rem',
                    margin: '0 0 8px 0'
                }}>
                    You do not have permission to view this page.
                </p>
                <p style={{
                    color: '#777777',
                    fontSize: '0.92rem',
                    margin: '0 0 20px 0'
                }}>
                    Please check your credentials and try again.
                </p>
                <p style={{
                    fontSize: '0.82rem',
                    margin: '0'
                }}>
                    <strong style={{ color: '#666666' }}>Error Code:</strong> 
                    <span style={{ color: '#888888', marginLeft: '4px' }}>403</span>
                </p>
            </div>
        </section>
    );
}
