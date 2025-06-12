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
          <div className="sms-form">

            <div id="block-token">
              <p>Je ontvangt een SMS van Yivi.</p>
              <b>Doorloop de volgende stappen:</b>
              <ol>
                <li>Open het SMS-bericht afkomstig van Yivi.</li>
                <li>Kies de link in het SMS-bericht.</li>
                <li>Je wordt teruggestuurd naar je Yivi-app.</li>
              </ol>
              <p>Bekijk je deze pagina niet op je mobiel? Vul hieronder de verificatiecode uit het SMS-bericht in.</p>
              <form id="token-form">
                <label htmlFor="submit-token">Verificatiecode</label>
                <input type="text" required className="form-control" pattern="[0-9A-Za-z]{6}"
                       />
                <button className="hidden" id="submit-token" type="submit"></button>
              </form>
            </div>
            
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
