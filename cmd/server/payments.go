package main

// import (
// 	"bytes"
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"log"
// 	"net/http"
// 	"net/url"
// 	"os"
// 	"strconv"
// 	"strings"

// 	"github.com/bsv-blockchain/go-sdk/overlay/lookup"
// 	"github.com/b-open-io/opns-overlay/opns"
// 	"github.com/gofiber/fiber/v2"
// )

// func registerPaymentRoutes(app *fiber.App) {

// 	// Stripe checkout session endpoint
// 	app.Post("/create-checkout-session", func(c *fiber.Ctx) error {
// 		// Get form values directly - no need to parse body first in Fiber
// 		productId := c.FormValue("productId", "")
// 		name := c.FormValue("name", "")
// 		priceStr := c.FormValue("price", "")
// 		successUrl := c.FormValue("success_url", "")
// 		cancelUrl := c.FormValue("cancel_url", "")
// 		address := c.FormValue("address", "")

// 		// Validate required fields
// 		if productId == "" || name == "" || priceStr == "" || successUrl == "" || cancelUrl == "" {
// 			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 				"error": "Missing required fields",
// 			})
// 		}

// 		// Parse price
// 		price, err := strconv.Atoi(priceStr)
// 		if err != nil || price <= 0 {
// 			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 				"error": "Invalid price",
// 			})
// 		}

// 		// Create the checkout session using Stripe API
// 		stripeKey := os.Getenv("STRIPE_SECRET_KEY")
// 		if stripeKey == "" {
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": "Stripe key not configured",
// 			})
// 		}

// 		// Set up the HTTP client
// 		client := &http.Client{}

// 		// Create form data for the Stripe API request
// 		formData := url.Values{}
// 		formData.Add("payment_method_types[]", "card")
// 		formData.Add("line_items[0][price_data][currency]", "usd")
// 		formData.Add("line_items[0][price_data][product_data][name]", fmt.Sprintf("%s@1sat.name", name))
// 		formData.Add("line_items[0][price_data][product_data][description]", "1sat Name Registration")
// 		formData.Add("line_items[0][price_data][unit_amount]", strconv.Itoa(price))
// 		formData.Add("line_items[0][quantity]", "1")
// 		formData.Add("metadata[name]", name)
// 		formData.Add("metadata[product_id]", productId)
// 		// Store the customer's address in metadata if provided
// 		if address != "" {
// 			formData.Add("metadata[address]", address)
// 		}
// 		formData.Add("mode", "payment")
// 		formData.Add("success_url", successUrl)
// 		formData.Add("cancel_url", cancelUrl)

// 		// Create the request
// 		req, err := http.NewRequest("POST", "https://api.stripe.com/v1/checkout/sessions", strings.NewReader(formData.Encode()))
// 		if err != nil {
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": "Failed to create request",
// 			})
// 		}

// 		// Set headers
// 		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
// 		req.Header.Set("Authorization", "Bearer "+stripeKey)

// 		// Send the request
// 		resp, err := client.Do(req)
// 		if err != nil {
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": "Failed to contact Stripe API",
// 			})
// 		}
// 		defer resp.Body.Close()

// 		// Parse the response
// 		var result map[string]any
// 		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": "Failed to parse Stripe response",
// 			})
// 		}

// 		// Check if there was an error
// 		if resp.StatusCode != http.StatusOK {
// 			errorMsg := "Unknown error"
// 			if errObj, ok := result["error"].(map[string]any); ok {
// 				if msg, ok := errObj["message"].(string); ok {
// 					errorMsg = msg
// 				}
// 			}
// 			return c.Status(resp.StatusCode).JSON(fiber.Map{
// 				"error": fmt.Sprintf("Stripe error: %s", errorMsg),
// 			})
// 		}

// 		// Return the checkout session URL
// 		return c.JSON(fiber.Map{
// 			"url": result["url"],
// 		})
// 	})

// 	// Name registration endpoint
// 	app.Post("/register", func(c *fiber.Ctx) error {
// 		// Get form values directly
// 		handle := c.FormValue("handle", "")
// 		address := c.FormValue("address", "")

// 		// Validate required fields
// 		if handle == "" {
// 			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 				"error": "Missing handle",
// 			})
// 		}

// 		// Check if the name is already registered (taken)
// 		question := &opns.Question{
// 			Event: "mine:" + handle,
// 		}

// 		b, err := json.Marshal(question)
// 		if err != nil {
// 			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 				"error": "Invalid question",
// 			})
// 		}

// 		answer, err := e.Lookup(c.Context(), &lookup.LookupQuestion{
// 			Service: "ls_OpNS",
// 			Query:   json.RawMessage(b),
// 		})

// 		// If we got an answer with outputs, the name is already taken
// 		if err == nil && len(answer.Outputs) > 0 {
// 			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
// 				"error": "Name is already registered",
// 			})
// 		}

// 		// Check if payment was made for this name
// 		paid := isNamePaid(c.Context(), handle)

// 		// If the name hasn't been paid for, require payment first
// 		if !paid {
// 			return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
// 				"error":   "Payment required for this name",
// 				"message": "Please complete payment before registering",
// 			})
// 		}

// 		// If address is not provided, try to get it from Redis
// 		if address == "" {
// 			storedAddress, err := getNameAddress(c.Context(), handle)
// 			if err == nil && storedAddress != "" {
// 				address = storedAddress
// 				log.Printf("Using stored address for %s: %s", handle, address)
// 			} else {
// 				log.Printf("No stored address found for %s, using default", handle)
// 				address = "1sat4utxoLYSZb3zvWH8vZ9ULhGbPZEPi6" // Default address
// 			}
// 		}

// 		// Log registration attempt
// 		log.Printf("Registering name: %s for address: %s", handle, address)

// 		// Call the actual mining API
// 		client := &http.Client{}
// 		miningPayload := map[string]string{
// 			"domain":       handle,
// 			"ownerAddress": address,
// 		}

// 		miningData, err := json.Marshal(miningPayload)
// 		if err != nil {
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": "Failed to prepare mining request",
// 			})
// 		}

// 		req, err := http.NewRequest("POST", "https://go-opns-mint-production.up.railway.app/mine", bytes.NewBuffer(miningData))
// 		if err != nil {
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": "Failed to create mining request",
// 			})
// 		}

// 		req.Header.Set("Content-Type", "application/json")

// 		resp, err := client.Do(req)
// 		if err != nil {
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": "Failed to call mining API: " + err.Error(),
// 			})
// 		}
// 		defer resp.Body.Close()

// 		// Handle mining API response
// 		if resp.StatusCode != http.StatusOK {
// 			log.Printf("Mining API returned error status: %s", resp.Status)
// 			return c.Status(resp.StatusCode).JSON(fiber.Map{
// 				"error": "Mining API returned error status: " + resp.Status,
// 			})
// 		}

// 		// The mining API returns a simple string which is the txid
// 		bodyBytes, err := io.ReadAll(resp.Body)
// 		if err != nil {
// 			log.Printf("Error reading mining API response: %v", err)
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": "Failed to read mining API response",
// 			})
// 		}

// 		// The response is just the txid as a string (remove any quotes or whitespace)
// 		txid := strings.Trim(string(bodyBytes), "\" \t\n\r")

// 		// Mark the name as successfully mined
// 		if err := markNameAsMined(c.Context(), handle, txid); err != nil {
// 			log.Printf("Error marking name as mined in Redis: %v", err)
// 			// Continue anyway - we still have the txid
// 		}

// 		log.Printf("Successfully registered name %s with txid %s", handle, txid)

// 		// Return success response
// 		return c.JSON(fiber.Map{
// 			"success":       true,
// 			"transactionId": txid,
// 			"name":          handle + "@1sat.name",
// 			"message":       "Name registration initiated",
// 		})
// 	})

// 	// Stripe webhook endpoint for handling payment events
// 	app.Post("/stripe-webhook", func(c *fiber.Ctx) error {
// 		// Get the stripe webhook secret from env
// 		webhookSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")
// 		if webhookSecret == "" {
// 			log.Println("Warning: STRIPE_WEBHOOK_SECRET not set")
// 		}

// 		// Verify the webhook signature
// 		payload := c.Body()
// 		// Signature is not currently validated but could be used with Stripe's SDK
// 		_ = c.Get("Stripe-Signature") // We acknowledge the signature but don't use it yet

// 		// For debugging
// 		log.Printf("Received webhook: %s", string(payload))

// 		// Parse the event
// 		var event map[string]interface{}
// 		if err := json.Unmarshal(payload, &event); err != nil {
// 			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 				"error": "Invalid JSON payload",
// 			})
// 		}

// 		// Get the event type
// 		eventType, ok := event["type"].(string)
// 		if !ok {
// 			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 				"error": "Missing event type",
// 			})
// 		}

// 		// Only process checkout.session.completed events
// 		if eventType != "checkout.session.completed" {
// 			log.Printf("Ignoring event of type: %s", eventType)
// 			return c.JSON(fiber.Map{
// 				"received": true,
// 			})
// 		}

// 		// Extract session data
// 		sessionData, ok := event["data"].(map[string]interface{})
// 		if !ok {
// 			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 				"error": "Invalid event data",
// 			})
// 		}

// 		session, ok := sessionData["object"].(map[string]interface{})
// 		if !ok {
// 			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 				"error": "Invalid session data",
// 			})
// 		}

// 		// Check payment status
// 		paymentStatus, ok := session["payment_status"].(string)
// 		if !ok || paymentStatus != "paid" {
// 			log.Printf("Payment not complete. Status: %s", paymentStatus)
// 			return c.JSON(fiber.Map{
// 				"received": true,
// 				"status":   "payment_incomplete",
// 			})
// 		}

// 		// Get metadata
// 		metadata, ok := session["metadata"].(map[string]interface{})
// 		if !ok {
// 			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 				"error": "Missing metadata",
// 			})
// 		}

// 		// Extract the name and customer details
// 		name, ok := metadata["name"].(string)
// 		if !ok || name == "" {
// 			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 				"error": "Missing name in metadata",
// 			})
// 		}

// 		// Get the address from metadata if available
// 		address, ok := metadata["address"].(string)
// 		if !ok || address == "" {
// 			// Fallback to default address if not provided
// 			address = "1sat4utxoLYSZb3zvWH8vZ9ULhGbPZEPi6"
// 			log.Printf("Using default address for %s: %s", name, address)
// 		}

// 		// Get customer details from the session
// 		customerDetails, ok := session["customer_details"].(map[string]interface{})
// 		if !ok {
// 			log.Println("Missing customer details")
// 		}

// 		// Get customer email or use a default
// 		var customerEmail string
// 		if email, ok := customerDetails["email"].(string); ok {
// 			customerEmail = email
// 		} else {
// 			customerEmail = "unknown@example.com"
// 		}

// 		log.Printf("Processing successful payment for name: %s, customer: %s", name, customerEmail)

// 		// Mark the name as paid in Redis
// 		if err := markNameAsPaid(c.Context(), name, address); err != nil {
// 			log.Printf("Error marking name as paid in Redis: %v", err)
// 			// Continue anyway - we still want to try registering the name
// 		}

// 		// Call the actual mining API
// 		client := &http.Client{}
// 		miningPayload := map[string]string{
// 			"domain":       name,
// 			"ownerAddress": address,
// 		}

// 		miningData, err := json.Marshal(miningPayload)
// 		if err != nil {
// 			log.Printf("Error preparing mining request: %v", err)
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": "Failed to prepare mining request",
// 			})
// 		}

// 		req, err := http.NewRequest("POST", "https://go-opns-mint-production.up.railway.app/mine", bytes.NewBuffer(miningData))
// 		if err != nil {
// 			log.Printf("Error creating mining request: %v", err)
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": "Failed to create mining request",
// 			})
// 		}

// 		req.Header.Set("Content-Type", "application/json")

// 		resp, err := client.Do(req)
// 		if err != nil {
// 			log.Printf("Error calling mining API: %v", err)
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": "Failed to call mining API: " + err.Error(),
// 			})
// 		}
// 		defer resp.Body.Close()

// 		// Handle mining API response
// 		if resp.StatusCode != http.StatusOK {
// 			log.Printf("Mining API returned error status: %s", resp.Status)
// 			return c.Status(resp.StatusCode).JSON(fiber.Map{
// 				"error": "Mining API returned error status: " + resp.Status,
// 			})
// 		}

// 		// The mining API returns a simple string which is the txid
// 		bodyBytes, err := io.ReadAll(resp.Body)
// 		if err != nil {
// 			log.Printf("Error reading mining API response: %v", err)
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": "Failed to read mining API response",
// 			})
// 		}

// 		// The response is just the txid as a string (remove any quotes or whitespace)
// 		txid := strings.Trim(string(bodyBytes), "\" \t\n\r")

// 		// Mark the name as successfully mined
// 		if err := markNameAsMined(c.Context(), name, txid); err != nil {
// 			log.Printf("Error marking name as mined in Redis: %v", err)
// 			// Continue anyway - we still have the txid
// 		}

// 		log.Printf("Successfully registered name %s with txid %s", name, txid)

// 		// Return success
// 		return c.JSON(fiber.Map{
// 			"received":      true,
// 			"success":       true,
// 			"name":          name + "@1sat.name",
// 			"transactionId": txid,
// 		})
// 	})

// }	// Direct registration with wallet payment endpoint
// app.Post("/register-direct", func(c *fiber.Ctx) error {
// 	var request struct {
// 		Name     string `json:"name"`
// 		Satoshis int64  `json:"satoshis"`
// 		Address  string `json:"address"` // Required payment address from frontend
// 	}

// 	if err := c.BodyParser(&request); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "Invalid request format",
// 		})
// 	}

// 	if request.Name == "" {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "Missing name parameter",
// 		})
// 	}

// 	if request.Satoshis <= 0 {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "Invalid payment amount",
// 		})
// 	}

// 	if request.Address == "" {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "Payment address is required",
// 		})
// 	}

// 	// Check if the name is already registered (taken)
// 	question := &opns.Question{
// 		Event: "mine:" + request.Name,
// 	}

// 	b, err := json.Marshal(question)
// 	if err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "Invalid question",
// 		})
// 	}

// 	answer, err := e.Lookup(c.Context(), &lookup.LookupQuestion{
// 		Service: "ls_OpNS",
// 		Query:   json.RawMessage(b),
// 	})

// 	// If we got an answer with outputs, the name is already taken
// 	if err == nil && len(answer.Outputs) > 0 {
// 		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
// 			"error": "Name is already registered",
// 		})
// 	}

// 	// Use the market address provided by the frontend
// 	paymentAddress := request.Address

// 	// Store the pending payment information in Redis
// 	if err := rdb.HSet(c.Context(), "pending_payments", request.Name, request.Satoshis).Err(); err != nil {
// 		log.Printf("Error storing pending payment: %v", err)
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to process payment request",
// 		})
// 	}

// 	// Return payment details to the client
// 	return c.JSON(fiber.Map{
// 		"address":  paymentAddress,
// 		"satoshis": request.Satoshis,
// 		"message":  "Send payment to complete registration",
// 	})
// })

// // Payment completion webhook
// app.Post("/payment-complete", func(c *fiber.Ctx) error {
// 	var request struct {
// 		Name    string `json:"name"`
// 		Txid    string `json:"txid"`
// 		Address string `json:"address"` // Added address field to accept from frontend
// 	}

// 	if err := c.BodyParser(&request); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "Invalid request format",
// 		})
// 	}

// 	if request.Name == "" || request.Txid == "" {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "Missing required parameters",
// 		})
// 	}

// 	// Verify the payment matches what was expected
// 	expectedSatoshis, err := rdb.HGet(c.Context(), "pending_payments", request.Name).Int64()
// 	if err != nil || expectedSatoshis <= 0 {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "No pending payment found for this name",
// 		})
// 	}

// 	// Use the address provided by the frontend if available
// 	// This should be their ordinals address from the wallet
// 	userAddress := request.Address
// 	if userAddress == "" {
// 		userAddress = "wallet-payment" // Fallback only if no address provided
// 		log.Printf("Warning: No address provided for %s, using placeholder", request.Name)
// 	} else {
// 		log.Printf("Using user's wallet address for %s: %s", request.Name, userAddress)
// 	}

// 	// Mark the name as paid in Redis with the user's wallet address
// 	if err := markNameAsPaid(c.Context(), request.Name, userAddress); err != nil {
// 		log.Printf("Error marking name as paid: %v", err)
// 	}

// 	// Now we need to register the name using the mining API
// 	// Get the address we just stored
// 	address, err := getNameAddress(c.Context(), request.Name)
// 	if err != nil || address == "" {
// 		log.Printf("Error getting stored address for %s: %v", request.Name, err)
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to retrieve payment address",
// 		})
// 	}

// 	// Call the actual mining API
// 	client := &http.Client{}
// 	miningPayload := map[string]string{
// 		"domain":       request.Name,
// 		"ownerAddress": address, // Use the address we stored (user's wallet address)
// 	}

// 	log.Printf("Registering name %s for address %s", request.Name, address)

// 	miningData, err := json.Marshal(miningPayload)
// 	if err != nil {
// 		log.Printf("Error preparing mining request: %v", err)
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to prepare mining request",
// 		})
// 	}

// 	req, err := http.NewRequest("POST", "https://go-opns-mint-production.up.railway.app/mine", bytes.NewBuffer(miningData))
// 	if err != nil {
// 		log.Printf("Error creating mining request: %v", err)
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to create mining request",
// 		})
// 	}

// 	req.Header.Set("Content-Type", "application/json")

// 	resp, err := client.Do(req)
// 	if err != nil {
// 		log.Printf("Error calling mining API: %v", err)
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to call mining API: " + err.Error(),
// 		})
// 	}
// 	defer resp.Body.Close()

// 	// Handle mining API response
// 	if resp.StatusCode != http.StatusOK {
// 		log.Printf("Mining API returned error status: %s", resp.Status)
// 		return c.Status(resp.StatusCode).JSON(fiber.Map{
// 			"error": "Mining API returned error status: " + resp.Status,
// 		})
// 	}

// 	// The mining API returns a simple string which is the txid
// 	bodyBytes, err := io.ReadAll(resp.Body)
// 	if err != nil {
// 		log.Printf("Error reading mining API response: %v", err)
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to read mining API response",
// 		})
// 	}

// 	// The response is just the txid as a string (remove any quotes or whitespace)
// 	miningTxid := strings.Trim(string(bodyBytes), "\" \t\n\r")

// 	// Mark name as mined in Redis
// 	if err := markNameAsMined(c.Context(), request.Name, miningTxid); err != nil {
// 		log.Printf("Error marking name as mined: %v", err)
// 	}

// 	log.Printf("Successfully registered name %s with mining txid %s and payment txid %s",
// 		request.Name, miningTxid, request.Txid)

// 	// Success response
// 	return c.JSON(fiber.Map{
// 		"success":    true,
// 		"name":       request.Name + "@1sat.name",
// 		"txid":       request.Txid, // Payment transaction ID
// 		"miningTxid": miningTxid,   // Mining transaction ID
// 		"message":    "Name registration successful",
// 	})
// })

// // Helper function to check if a name has been paid for
// func isNamePaid(ctx context.Context, name string) bool {
// 	result, err := rdb.HGet(ctx, "paid_names", name).Bool()
// 	if err != nil {
// 		return false
// 	}
// 	return result
// }

// // Helper function to mark a name as paid
// func markNameAsPaid(ctx context.Context, name string, address string) error {
// 	// Store name payment status
// 	if err := rdb.HSet(ctx, "paid_names", name, true).Err(); err != nil {
// 		return err
// 	}

// 	// Store customer address for this name
// 	if err := rdb.HSet(ctx, "name_addresses", name, address).Err(); err != nil {
// 		return err
// 	}

// 	return nil
// }

// // Helper function to mark a name as successfully mined
// func markNameAsMined(ctx context.Context, name string, txid string) error {
// 	// Store the transaction ID
// 	if err := rdb.HSet(ctx, "name_txids", name, txid).Err(); err != nil {
// 		return err
// 	}

// 	return nil
// }

// // Helper function to get the address for a name
// func getNameAddress(ctx context.Context, name string) (string, error) {
// 	address, err := rdb.HGet(ctx, "name_addresses", name).Result()
// 	return address, err
// }
