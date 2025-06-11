import { useState, useEffect } from 'react';
import Cookies from 'js-cookie';
import jwtDecode from 'jwt-decode';
import { useTranslation } from 'react-i18next';
import { useNavigate } from "react-router-dom";

interface CredAttr {
  [key: string]: string;
}

export default function EnrollPage() {
  const navigate = useNavigate();
  const { t } = useTranslation();

  const enroll = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();

    // import("@privacybydesign/yivi-frontend").then((yivi) => {
    //   yivi.newPopup({
    //     language: config.language,
    //     session: {
    //       url: config.irma_server_url,
    //       start: {
    //         method: 'POST',
    //         headers: { 'Content-Type': 'text/plain' },
    //         body: Cookies.get('jwt')!,
    //       },
    //       result: false,
    //     },
    //   })
    //   .start()
    //   .then(() => {
    //     navigate("/");
    //   });
    // });
  };

  return (
    <>
      <form id="container" onSubmit={enroll}>
        <header>
          <h1>{t('index_header')}</h1>
        </header>
        <main>
          <div id="idin-form">

            <p>{t('enroll_received_attributes')}</p>

            <p>{t('enroll_derived_attributes')}</p>
            
          </div>
        </main>
        <footer>
          <div className="actions">
            <div></div>
            <button id="submit-button" >{t('enroll_load_button')}</button>
          </div>
        </footer>
      </form>
    </>
  );
}
