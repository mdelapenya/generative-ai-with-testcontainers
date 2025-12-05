package main

import "fmt"

// fibonacci calculates the nth Fibonacci number iteratively
func fibonacci(n int) int {
	if n <= 1 {
		return n
	}

	prev, curr := 0, 1
	for i := 2; i <= n; i++ {
		prev, curr = curr, prev+curr
	}

	return curr
}

func main() {
	// Print the first 10 Fibonacci numbers
	fmt.Println("First 10 Fibonacci numbers:")
	for i := 0; i < 10; i++ {
		fmt.Printf("F(%d) = %d\n", i, fibonacci(i))
	}
}
