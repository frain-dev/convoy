/*!-----------------------------------------------------------------------------
 * Copyright (c) Microsoft Corporation. All rights reserved.
 * Version: 0.34.1(547870b6881302c5b4ff32173c16d06009e3588f)
 * Released under the MIT license
 * https://github.com/microsoft/monaco-editor/blob/main/LICENSE.txt
 *-----------------------------------------------------------------------------*/

// src/basic-languages/dockerfile/dockerfile.ts
var conf = {
  brackets: [
    ["{", "}"],
    ["[", "]"],
    ["(", ")"]
  ],
  autoClosingPairs: [
    { open: "{", close: "}" },
    { open: "[", close: "]" },
    { open: "(", close: ")" },
    { open: '"', close: '"' },
    { open: "'", close: "'" }
  ],
  surroundingPairs: [
    { open: "{", close: "}" },
    { open: "[", close: "]" },
    { open: "(", close: ")" },
    { open: '"', close: '"' },
    { open: "'", close: "'" }
  ]
};
var language = {
  defaultToken: "",
  tokenPostfix: ".dockerfile",
  variable: /\${?[\w]+}?/,
  tokenizer: {
    root: [
      { include: "@whitespace" },
      { include: "@comment" },
      [/(ONBUILD)(\s+)/, ["keyword", ""]],
      [/(ENV)(\s+)([\w]+)/, ["keyword", "", { token: "variable", next: "@arguments" }]],
      [
        /(FROM|MAINTAINER|RUN|EXPOSE|ENV|ADD|ARG|VOLUME|LABEL|USER|WORKDIR|COPY|CMD|STOPSIGNAL|SHELL|HEALTHCHECK|ENTRYPOINT)/,
        { token: "keyword", next: "@arguments" }
      ]
    ],
    arguments: [
      { include: "@whitespace" },
      { include: "@strings" },
      [
        /(@variable)/,
        {
          cases: {
            "@eos": { token: "variable", next: "@popall" },
            "@default": "variable"
          }
        }
      ],
      [
        /\\/,
        {
          cases: {
            "@eos": "",
            "@default": ""
          }
        }
      ],
      [
        /./,
        {
          cases: {
            "@eos": { token: "", next: "@popall" },
            "@default": ""
          }
        }
      ]
    ],
    whitespace: [
      [
        /\s+/,
        {
          cases: {
            "@eos": { token: "", next: "@popall" },
            "@default": ""
          }
        }
      ]
    ],
    comment: [[/(^#.*$)/, "comment", "@popall"]],
    strings: [
      [/\\'$/, "", "@popall"],
      [/\\'/, ""],
      [/'$/, "string", "@popall"],
      [/'/, "string", "@stringBody"],
      [/"$/, "string", "@popall"],
      [/"/, "string", "@dblStringBody"]
    ],
    stringBody: [
      [
        /[^\\\$']/,
        {
          cases: {
            "@eos": { token: "string", next: "@popall" },
            "@default": "string"
          }
        }
      ],
      [/\\./, "string.escape"],
      [/'$/, "string", "@popall"],
      [/'/, "string", "@pop"],
      [/(@variable)/, "variable"],
      [/\\$/, "string"],
      [/$/, "string", "@popall"]
    ],
    dblStringBody: [
      [
        /[^\\\$"]/,
        {
          cases: {
            "@eos": { token: "string", next: "@popall" },
            "@default": "string"
          }
        }
      ],
      [/\\./, "string.escape"],
      [/"$/, "string", "@popall"],
      [/"/, "string", "@pop"],
      [/(@variable)/, "variable"],
      [/\\$/, "string"],
      [/$/, "string", "@popall"]
    ]
  }
};
export {
  conf,
  language
};
