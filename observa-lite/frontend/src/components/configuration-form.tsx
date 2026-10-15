import {
  Children,
  createContext,
  cloneElement,
  isValidElement,
  useContext,
  useId,
  useState,
  type ComponentProps,
  type ReactNode,
} from "react";

const ConfigurationContext = createContext(false);

/** Shared configuration layout; native form validation and submission stay intact. */
export function ConfigurationForm({
  children,
  className = "",
  ...props
}: ComponentProps<"form">) {
  return (
    <ConfigurationContext.Provider value={true}>
      <form {...props} className={`configuration-form ${className}`}>
        <p className="configuration-legend">
          <span aria-hidden="true">*</span> Campos com * são obrigatórios. Os
          demais são opcionais.
        </p>
        {children}
      </form>
    </ConfigurationContext.Provider>
  );
}

export function ConfigurationSection({
  children,
  className = "",
  ...props
}: ComponentProps<"div">) {
  return (
    <ConfigurationContext.Provider value={true}>
      <div {...props} className={`configuration-form ${className}`}>
        {children}
      </div>
    </ConfigurationContext.Provider>
  );
}

type ControlProps = {
  children?: ReactNode;
  required?: boolean;
  type?: string;
  value?: unknown;
  id?: string;
};
function controls(node: ReactNode): ControlProps[] {
  return Children.toArray(node).flatMap((child) => {
    if (!isValidElement<ControlProps>(child)) return [];
    const p = child.props;
    if (typeof child.type === "string" && ["span", "div"].includes(child.type))
      return controls(p.children);
    if (
      "value" in p ||
      "required" in p ||
      child.type === "input" ||
      child.type === "textarea" ||
      child.type === "select"
    )
      return [p];
    return controls(p.children);
  });
}

export function ConfigField({
  children,
  className = "",
  ...props
}: ComponentProps<"label">) {
  const configured = useContext(ConfigurationContext);
  const generatedId = useId();
  const fields = controls(children);
  if (
    !configured ||
    !fields.length ||
    fields.some((f) => f.type === "checkbox" || f.type === "radio")
  )
    return (
      <label {...props} className={className}>
        {children}
      </label>
    );
  const parts = Children.toArray(children);
  const firstControl = parts.findIndex(
    (c) =>
      isValidElement<ControlProps>(c) &&
      ("value" in c.props ||
        "required" in c.props ||
        ["input", "textarea", "select"].includes(String(c.type))),
  );
  if (firstControl < 1)
    return (
      <label {...props} className={className}>
        {children}
      </label>
    );
  const required = fields.some((f) => f.required);
  const control = parts[firstControl];
  const controlId = isValidElement<ControlProps>(control)
    ? (control.props.id ?? generatedId)
    : generatedId;
  return (
    <div className={`configuration-field ${className}`}>
      <span className="configuration-caption">
        <label {...props} htmlFor={controlId}>
          {parts.slice(0, firstControl)}
        </label>
        <span
          aria-hidden="true"
          className={
            required ? "configuration-required" : "configuration-optional"
          }
        >
          {required ? "* Obrigatório" : "Opcional"}
        </span>
      </span>
      <span className="configuration-control">
        {parts
          .slice(firstControl)
          .map((part, index) =>
            index === 0 && isValidElement<ControlProps>(part)
              ? cloneElement(part, { id: controlId })
              : part,
          )}
      </span>
    </div>
  );
}

export function ConfigurationTabs({
  sections,
}: {
  sections: { id: string; title: string; content: ReactNode }[];
}) {
  const [active, setActive] = useState(sections[0]?.id ?? "");
  const prefix = useId();
  return (
    <div
      className="configuration-tabs"
      onInvalidCapture={(event) => {
        const target = event.target as HTMLElement;
        const panel = target.closest<HTMLElement>("[data-config-section]");
        if (panel?.hidden && panel.dataset["configSection"]) {
          event.preventDefault();
          setActive(panel.dataset["configSection"]);
          requestAnimationFrame(() => {
            target.focus();
            (target as HTMLInputElement).reportValidity();
          });
        }
      }}
    >
      <div
        role="tablist"
        aria-label="Seções de configuração"
        className="configuration-tablist"
      >
        {sections.map((section, index) => (
          <button
            key={section.id}
            id={`${prefix}-tab-${section.id}`}
            type="button"
            role="tab"
            aria-selected={active === section.id}
            aria-controls={`${prefix}-panel-${section.id}`}
            tabIndex={active === section.id ? 0 : -1}
            onClick={() => setActive(section.id)}
            onKeyDown={(event) => {
              const offset =
                event.key === "ArrowRight"
                  ? 1
                  : event.key === "ArrowLeft"
                    ? -1
                    : 0;
              if (!offset && !["Home", "End"].includes(event.key)) return;
              event.preventDefault();
              const next =
                event.key === "Home"
                  ? 0
                  : event.key === "End"
                    ? sections.length - 1
                    : (index + offset + sections.length) % sections.length;
              const section = sections[next];
              if (!section) return;
              setActive(section.id);
              document.getElementById(`${prefix}-tab-${section.id}`)?.focus();
            }}
          >
            {section.title}
          </button>
        ))}
      </div>
      {sections.map((section) => (
        <div
          key={section.id}
          role="tabpanel"
          id={`${prefix}-panel-${section.id}`}
          aria-labelledby={`${prefix}-tab-${section.id}`}
          data-config-section={section.id}
          hidden={active !== section.id}
          className="configuration-panel"
        >
          {section.content}
        </div>
      ))}
    </div>
  );
}
