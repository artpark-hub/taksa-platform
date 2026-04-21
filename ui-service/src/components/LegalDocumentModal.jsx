'use client';

import React from 'react';
import { X } from 'lucide-react';
import { LEGAL_DOCUMENTS } from './legalContent';

const LegalDocumentBody = ({ documentKey }) => {
  const document = LEGAL_DOCUMENTS[documentKey];

  return (
    <>
      <div className="legal-document-header-block">
        <h2 id={`legal-title-${documentKey}`} className="legal-document-title">{document.title}</h2>
      </div>

      <div className="legal-document-content">
        {document.intro.map((paragraph) => (
          <p key={paragraph} className="legal-document-paragraph">{paragraph}</p>
        ))}

        {document.sections.map((section, index) => (
          <section key={section.title} className="legal-document-section">
            <h3 className="legal-document-section-title">{index + 1}. {section.title}</h3>
            {section.paragraphs?.map((paragraph) => (
              <p key={paragraph} className="legal-document-paragraph">{paragraph}</p>
            ))}
            {section.bullets?.length ? (
              <ul className="legal-document-list">
                {section.bullets.map((bullet) => (
                  <li key={bullet}>{bullet}</li>
                ))}
              </ul>
            ) : null}
            {section.contactEmail ? (
              <a className="legal-document-contact" href={`mailto:${section.contactEmail}`}>
                {section.contactEmail}
              </a>
            ) : null}
          </section>
        ))}
      </div>
    </>
  );
};

export const LegalDocumentPage = ({ documentKey }) => (
  <div className="legal-page-shell">
    <div className="legal-modal-card legal-page-card">
      <div className="legal-modal-scroll-area legal-page-scroll-area">
        <LegalDocumentBody documentKey={documentKey} />
      </div>
    </div>
  </div>
);

const LegalDocumentModal = ({ documentKey, open, onClose }) => {
  if (!open) return null;

  return (
    <div className="legal-modal-overlay" role="dialog" aria-modal="true" aria-labelledby={`legal-title-${documentKey}`}>
      <div className="legal-modal-card">
        <button type="button" className="legal-modal-close-icon" onClick={onClose} aria-label="Close dialog">
          <X size={24} />
        </button>

        <div className="legal-modal-scroll-area">
          <LegalDocumentBody documentKey={documentKey} />
        </div>

        <div className="legal-modal-footer">
          <p className="legal-modal-footer-copy">Read this document and close the modal when you are done.</p>
        </div>
      </div>
    </div>
  );
};

export default LegalDocumentModal;
