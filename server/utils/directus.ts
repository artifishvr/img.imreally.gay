import { createDirectus, staticToken, rest } from "@directus/sdk";

const DIRECTUS_URL = process.env.DIRECTUS_URL;
const DIRECTUS_TOKEN = process.env.DIRECTUS_TOKEN;

interface Bodies {
  id: string;
  sessions: number;
  gender: string;
  date_updated: string;
  date_created: string;
}

interface Pictures {
  id: number;
  name: string;
  date_updated: string;
  date_created: string;
  picture: string;
  vrc_ID: string;
}

interface Schema {
  bodies: Bodies[];
  thewall: Pictures[];
}

const client = createDirectus<Schema>(DIRECTUS_URL)
  .with(staticToken(DIRECTUS_TOKEN))
  .with(rest());

export default client;
