# Примеры

В репозитории есть несколько разных сценариев использования `witgo`:

- [generate](generate/README.md): генерация Go bindings из WIT-контракта;
- [plugin](plugin/README.md): общий бинарный component для базового сценария;
- [server](server/main.go): типизированный host, который открывает plugin и
  читает metadata через generated client;
- [validate](validate/main.go): предзапусковая проверка контракта и короткий
  startup-check;
- [inspect](inspect/main.go): инспекция imports, exports и structural
  signatures без instantiation;
- [collections](collections/main.go): отдельный self-contained component с
  `map`, вложенными `list` и `variant`;
- [handles](handles/main.go): отдельный self-contained component с живым
  `resource` handle.

Структура теперь единая:

- у сценария есть свой `main.go`;
- если сценарию нужен собственный component, рядом лежат `component.wat` и
  `component.wasm`;
- если несколько сценариев используют один и тот же component, он вынесен в
  отдельный каталог `plugin`.
