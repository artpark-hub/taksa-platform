import type { Metadata } from "next";
import { Inter } from "next/font/google";
import "./globals.css";
import "../components/Login.css";
import "../components/Register.css";
import "../components/ForgotPassword.css";
import "../components/LegalDocuments.css";

const inter = Inter({ subsets: ["latin"] });

export const metadata: Metadata = {
  title: "Taksa",
  description: "Open Source Factory OS",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body className={inter.className}>
        <script
          dangerouslySetInnerHTML={{
            __html: `(function(){try{var t=localStorage.getItem('taksa_theme');document.documentElement.dataset.theme=t==='dark'?'dark':'light';document.documentElement.style.colorScheme=document.documentElement.dataset.theme;}catch(e){document.documentElement.dataset.theme='light';}})();`,
          }}
        />
        {children}
      </body>
    </html>
  );
}