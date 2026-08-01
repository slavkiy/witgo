# Вложенный host-компонент

`host.component.wasm` экспортирует `examples:contract/host@1.0.0#process-string`.
Root-компонент `../component.wasm` импортирует эту функцию, поэтому runtime
находит dependency автоматически без реализации Go interface `Host`.
