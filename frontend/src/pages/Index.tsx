import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  PhoneInput,
  defaultCountries,
  parseCountry,
  CountryData,
} from 'react-international-phone';
import 'react-international-phone/style.css';
import { parsePhoneNumberFromString } from 'libphonenumber-js';
import { useNavigate } from "react-router-dom";
import { useAppContext } from '../AppContext';

const isPhoneValid = (input: string) => {
  try {
    const phone = parsePhoneNumberFromString(input);
    return phone?.isValid();
  } catch (error) {
    return false;
  }
};

export default function IndexPage() {
  const { t, i18n } = useTranslation();
  const { phoneNumber, setPhoneNumber} = useAppContext();
  const isValid = isPhoneValid(phoneNumber || '');
  const [showError, setShowError] = useState(false);
  const navigate = useNavigate();

  // Excluded countries must match the irmamobile app's phone number entry screen.
  const excludedCountries = new Set([
    'af', // Afghanistan
    'ao', // Angola
    'dz', // Algeria
    'az', // Azerbaijan
    'bd', // Bangladesh
    'by', // Belarus
    'bt', // Bhutan
    'bi', // Burundi
    'eg', // Egypt
    'et', // Ethiopia
    'id', // Indonesia
    'ir', // Iran
    'iq', // Iraq
    'jo', // Jordan
    'kz', // Kazakhstan
    'xk', // Kosovo
    'kg', // Kyrgyzstan
    'lb', // Lebanon
    'ly', // Libya
    'mg', // Madagascar
    'mw', // Malawi
    'mr', // Mauritania
    'np', // Nepal
    'pk', // Pakistan
    'ru', // Russia
    'sn', // Senegal
    'si', // Slovenia
    'lk', // Sri Lanka
    'sy', // Syria
    'tj', // Tajikistan
    'tz', // Tanzania
    'tn', // Tunisia
    'tm', // Turkmenistan
    'uz', // Uzbekistan
    'ye', // Yemen
  ]);

  // Sint Maarten is not in the react-international-phone default list but is
  // available in the irmamobile app, so we add it as a custom country entry.
  const sintMaarten: CountryData = ['Sint Maarten', 'sx', '1721'];

  const countries = [...defaultCountries, sintMaarten].filter((country) => {
    const { iso2 } = parseCountry(country);
    return !excludedCountries.has(iso2);
  });

  const onChange = (value: string) => {
    setPhoneNumber(value);
    if (showError && isValid) {
      setShowError(false);
    }
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!isValid) {
      setShowError(true);
      return;
    }
    navigate(`/${i18n.language}/validate`);
  };

  return (
    <>
      <form id="container" onSubmit={submit}>
        <header>
          <h1>{t('index_header')}</h1>
        </header>
        <main>
          <div className="sms-form">
            <p>{t('index_explanation')}</p>
            <p>{t('index_multiple_numbers')}</p>
            <label htmlFor="bank-select">{t('phone_number')}</label>
            <PhoneInput
              defaultCountry="nl"
              value={phoneNumber}
              onChange={onChange}
              countries={countries}
              autoFocus
            />
            <p>
              {showError && <div className="warning">{t('index_phone_not_valid')}</div>}
            </p>
          </div>
        </main>
        <footer>
          <div className="actions">
            <div></div>
            <button id="submit-button" type="submit">{t('index_start')}</button>
          </div>
        </footer>
      </form>
    </>
  );
}
