import random

def play_game():
    target = random.randint(0, 100)
    print("Welcome to the Guessing Game!")
    print("I have picked a secret number between 0 and 100.")

    while True:
        try:
            guess = int(input("Enter your guess: "))
            if guess == target:
                print("Congratulations! You guessed the number correctly. You won!")
                break
            elif guess < target:
                print("bigger")
            else:
                print("smaller")
        except ValueError:
            print("Invalid input. Please enter a number.")

if __name__ == "__main__":
    play_game()