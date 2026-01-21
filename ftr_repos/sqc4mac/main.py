from tkinter import *
from tkmacosx import Button
from tkinter import messagebox
import tkinter.font as tkfont
import sympy as sp
import re

def show_help():
    help_message = (
        "Squ1dCalc4Mac v2, written by Quan Thai:\n\n"
        "Basic Operations:\n"
        "  Use numbers and operators (+, -, *, /) for basic calculations.\n\n"
        "Simultaneous Equations:\n"
        "  Write multiple equations separated by new lines.\n"
        "  Example:\n"
        "    x-2=9\n"
        "    x=y+6\n"
        "    y=8\n\n"
        "Variable Assignment:\n"
        "  Assign values to variables using '='.\n"
        "  Example: a=5\n\n"
        "Buttons:\n"
        "  'C' clears the input field.\n"
        "  '⏎' evaluates the expression.\n"
        "  '↓' inserts a new line.\n"
    )
    messagebox.showinfo("Help", help_message)

variables = {}
mode = "Simplify"

def insert_multiplication(expression):
    # Automatically insert multiplication between numbers/variables/parentheses
    expression = re.sub(r'(?<=\d)([a-zA-Z(])', r'*\1', expression)
    expression = re.sub(r'(?<=[a-zA-Z)])\(', r'*(', expression)
    expression = re.sub(r'(?<=[a-zA-Z])\d', r'*\g<0>', expression)
    return expression

def evaluate_expression():
    try:
        expression = entry.get("1.0", "end-1c").strip()
        global variables

        lines = [line.strip() for line in expression.split("\n") if line.strip()]

        if len(lines) == 1:
            expr = insert_multiplication(lines[0])

            if "=" in expr:
                lhs, rhs = map(str.strip, expr.split("="))
                lhs_expr = sp.sympify(lhs, locals=variables)
                rhs_expr = sp.sympify(rhs, locals=variables)

                symbols = lhs_expr.free_symbols.union(rhs_expr.free_symbols)

                if symbols:
                    solution = sp.solve(lhs_expr - rhs_expr, list(symbols)[0])
                    result = f"{list(symbols)[0]} = {solution}"
                else:
                    result = "No variables to solve for."

            else:
                if re.search(r"[a-zA-Z]", expr):
                    result = str(sp.simplify(sp.sympify(expr, locals=variables))) if mode == "Simplify" else str(sp.sympify(expr, locals=variables).evalf())
                else:
                    simp = sp.sympify(expr, locals=variables)
                    result = str(simp) if mode == "Simplify" else str(float(simp.evalf()))

        else:
            equations = []
            symbols = set()
            for line in lines:
                lhs, rhs = map(str.strip, insert_multiplication(line).split("="))
                lhs_expr = sp.sympify(lhs, locals=variables)
                rhs_expr = sp.sympify(rhs, locals=variables)

                equations.append(lhs_expr - rhs_expr)
                symbols.update(lhs_expr.free_symbols)
                symbols.update(rhs_expr.free_symbols)

            solution = sp.solve(equations, list(symbols))

            if isinstance(solution, dict):
                result = "\n".join([f"{key} = {val}" for key, val in solution.items()])
            elif isinstance(solution, list):
                result = ", ".join(map(str, solution))
            else:
                result = str(solution)

        entry.delete("1.0", "end-1c")
        entry.insert("1.0", expression + "\n" + result)

    except Exception as e:
        messagebox.showerror("Error", f"Invalid Input: {e}")

def clear():
    entry.delete("1.0", "end-1c")

def toggle_mode():
    global mode
    mode = "Evaluate" if mode == "Simplify" else "Simplify"
    mode_button.config(text=f"Mode: {mode}")

root = Tk()
root.title("Squ1dCalc")
root.configure(bg="#222222")
root.resizable(False, False)
root.attributes("-fullscreen", False)

text_frame = Frame(root, bg="#222222")
text_frame.pack(pady=5, padx=5, fill="both", expand=True)

entry = Text(
    text_frame,
    font=("Helvetica Neue", 20) if "Helvetica Neue" in tkfont.families() else ("Helvetica", 20),
    width=20, height=5,
    fg="#ffffff", bg="#222222",
    insertbackground="#ffffff",
    wrap="none", bd=0, highlightthickness=0
)
entry.pack(padx=5, pady=5, fill="both", expand=True)

buttons_frame = Frame(root, bg="#222222")
buttons = [
    ("7", "8", "9", "/"),
    ("4", "5", "6", "*"),
    ("1", "2", "3", "-"),
    ("0", ".", "=", "+"),
    ("(", ")", "⏎", "√"),
    ("x", "y", "z", "↓"),
    ("?", "⌫", "AC", "^")
]

def on_button_click(value):
    current = entry.get("1.0", "end-1c")
    if value == "⏎":
        evaluate_expression()
    elif value == "AC":
        clear()
    elif value == "↓":
        entry.delete("1.0", "end-1c")
        entry.insert("1.0", current + "\n")
    elif value == "√":
        entry.delete("1.0", "end-1c")
        entry.insert("1.0", current + "sqrt(")
    elif value == "?":
        show_help()
    elif value == "⌫":
        entry.delete("end-2c", "end-1c")
    else:
        entry.delete("1.0", "end-1c")
        entry.insert("1.0", current + value)

# Create all buttons with equal width
for row in buttons:
    button_row = Frame(buttons_frame, bg="#1e1e1e")
    for button in row:
        btn = Button(
            button_row, text=button,
            bg="#2e2e2e", fg="#ffffff",
            activebackground="#555555", activeforeground="#ffffff",
            font=("Helvetica Neue", 20),
            borderless=1,
            width=80, height=60,
            command=lambda value=button: on_button_click(value)
        )
        btn.pack(side="left", padx=2, pady=2)
    button_row.pack()
buttons_frame.pack()

mode_button = Button(
    root, text=f"Mode: {mode}",
    bg="#2e2e2e", fg="#ffffff",
    activebackground="#555555", activeforeground="#ffffff",
    font=("Helvetica Neue", 14),
    borderless=1,
    width=320, height=40,
    command=toggle_mode
)
mode_button.pack(pady=6)

root.bind("<Insert>", lambda event: evaluate_expression())
root.bind("<Shift-Return>", lambda event: evaluate_expression())
root.bind("<Escape>", lambda event: clear())

root.mainloop()
