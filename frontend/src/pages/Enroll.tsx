import { useTranslation } from 'react-i18next';
import { Link, useNavigate } from "react-router-dom";
import { useAppContext } from '../AppContext';
import i18n from '../i18n';
import { useEffect, useState } from 'react';
import parsePhoneNumberFromString from 'libphonenumber-js';

type VerifyResponse = {
  jwt: string;
  irma_server_url: string;
};

export default function EnrollPage() {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const [message, setMessage] = useState<string | undefined>(undefined);
  const [errorMessage, setErrorMessage] = useState<string | undefined>(undefined);
  const { phoneNumber, setPhoneNumber } = useAppContext();


  useEffect(() => {
    setMessage(t("sms_sent"));
  }, [phoneNumber]);


  // whether the user types the verification code or with the deeplink in the SMS we verify and start the issuance
  const VerifyAndStartIssuance = async (phoneNumber: string, token: string) => {
  const parsedPhoneNumber = parsePhoneNumberFromString(phoneNumber);

  try {
  // send phone number and token to verify endpoint to see if the token is valid for this phone number
  const response = await fetch("/verify", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      phone: parsedPhoneNumber?.number,
      token: token,
    }),
  });

  if (response.ok) {
    // Start enrollment process
    const res: VerifyResponse = await response.json();
    import("@privacybydesign/yivi-frontend").then((yivi) => {
      const issuance = yivi.newPopup({
        language: i18n.language,
        session: {
          url: res.irma_server_url,
          start: {
            method: "POST",
            headers: {
              "Content-Type": "text/plain",
            },
            body: res.jwt,
          },
          result: false,
        },
      });
      issuance
        .start()
        .then(() => {
          setMessage(t("phone_add_success"));
          setPhoneNumber("");
          navigate(`/${i18n.language}/done`);
        })
        .catch((e: string) => {
          if (e === "Aborted") {
            setErrorMessage(t("phone_add_cancel"));
          } else {
            setErrorMessage(t("phone_add_error"));
          }
        });
    });
    return;
  }} catch (error) {
    navigate(`/${i18n.language}/error`);
  }
};

 // user types the verification code
  const enroll = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setErrorMessage(undefined);

    // Get the token from the input field
    const tokenInput = document.querySelector('.verification-code-input') as HTMLInputElement;
    const token = tokenInput.value.trim();
    if (!token || token.length !== 6 || !phoneNumber) {
      navigate(`/${i18n.language}/error`);
      return;
    }

    await VerifyAndStartIssuance(phoneNumber, token);
  };

  // if the user has a deeplink in the SMS we verify and start the issuance
  useEffect(() => {
    const hash = window.location.hash;
    const match = hash.match(/^#!verify:(\+\d+):([0-9A-Za-z]{6})$/);
    if (!match) {
      return;
    }
    const [, linkPhoneNumber, linkToken] = match;
    VerifyAndStartIssuance(linkPhoneNumber, linkToken);
  }, [navigate, setPhoneNumber, t]);

  return (
    <>
      <form id="container" onSubmit={enroll}>
        <header>
          <h1>{t("index_header")}</h1>
        </header>
        <main>
          <div className="sms-form">
            <div id="block-token">
              {!errorMessage && message && (
                <div
                  id="status-bar"
                  className="alert alert-success"
                  role="alert"
                >
                  <div className="status-container">
                    <div id="status">{message}</div>
                  </div>
                </div>
              )}
              {errorMessage && (
                <div
                  id="status-bar"
                  className="alert alert-danger"
                  role="alert"
                >
                  <div className="status-container">
                    <div id="status">{errorMessage}</div>
                  </div>
                </div>
              )}
              <p>{t("receive_sms")}</p>
              <b>{t("steps")}</b>
              <ol>
                <li>{t("step_1")}</li>
                <li>{t("step_2")}</li>
                <li>{t("step_3")}</li>
              </ol>
              <p>{t("not_mobile")}</p>
              <label htmlFor="submit-token">{t("verification_code")}</label>
              <input
                type="text"
                required
                className="form-control verification-code-input"
                pattern="[0-9A-Za-z]{6}"
                style={{ textTransform: "uppercase" }}
                onInput={(e) => {
                  const input = e.target as HTMLInputElement;
                  input.value = input.value.toUpperCase();
                }}
              />

              <button
                className="hidden"
                id="submit-token"
                type="submit"
              ></button>
            </div>
          </div>
        </main>
        <footer>
          <div className="actions">
            <Link to={`/${i18n.language}/validate`} id="back-button">
              {t("back")}
            </Link>
            <button id="submit-button">{t("verify")}</button>
          </div>
        </footer>
      </form>
    </>
  );
}
