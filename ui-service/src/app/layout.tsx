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
            __html: `(function(){try{var t=localStorage.getItem('taksa_theme');var theme=t==='dark'?'dark':'light';document.documentElement.dataset.theme=theme;document.documentElement.style.colorScheme=theme;}catch(e){document.documentElement.dataset.theme='light';document.documentElement.style.colorScheme='light';}})();`,
          }}
        />
        {children}
      </body>
    </html>
  );
}