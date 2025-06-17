import { useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { AppProvider } from "./AppContext";

import IndexPage from './pages/Index';
import ValidatePage from './pages/Validate';
import EnrollPage from './pages/Enroll';
import ErrorPage from './pages/Error';
import DonePage from './pages/Done';

import './i18n';

// Wrapper that sets the language based on the URL
function LanguageRouter() {
  const { lang } = useParams();
  const { i18n } = useTranslation();

  useEffect(() => {
    if (lang && i18n.language !== lang) {
      i18n.changeLanguage(lang);
    }
  }, [lang, i18n]);

  return (
    <Routes>
      <Route path="/" element={<IndexPage />} />
      <Route path="/enroll" element={<EnrollPage />} />
      <Route path="/validate" element={<ValidatePage />} />
      <Route path="/error" element={<ErrorPage />} />
      <Route path="/done" element={<DonePage />} />
    </Routes>
  );
}

function App() {
  return (
    <AppProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/" element={<Navigate to="/nl" replace />} />

          <Route path=":lang/*" element={<LanguageRouter />} />
        </Routes>
      </BrowserRouter>
    </AppProvider>
  );
}

export default App;